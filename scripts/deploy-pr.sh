#!/bin/bash
# 为一个 PR 部署独立预览环境:
#   ECS 服务 app-pr-N + 目标组 tg-pr-N + ALB 按域名分流规则 + Cloudflare DNS
set -euo pipefail

HOST="pr-${PR_NUMBER}.${DOMAIN}"

# ── 1. 换上"部署角色"(考点:构建角色只能推镜像,部署动作必须 AssumeRole)──
CREDS=$(aws sts assume-role --role-arn "$DEPLOY_ROLE_ARN" \
  --role-session-name "deploy-pr-${PR_NUMBER}" \
  --query 'Credentials.[AccessKeyId,SecretAccessKey,SessionToken]' --output text)
export AWS_ACCESS_KEY_ID=$(echo "$CREDS" | awk '{print $1}')
export AWS_SECRET_ACCESS_KEY=$(echo "$CREDS" | awk '{print $2}')
export AWS_SESSION_TOKEN=$(echo "$CREDS" | awk '{print $3}')
echo "assumed deploy role"

# ── 2. 注册任务定义:以线上 go-profile 为模板,换镜像 + 换独立数据库 ──
# 每个 PR 用自己的库 profile_pr_N(同一 RDS 实例内多库免费,数据互不干扰;
# 库不存在时 Go 程序启动会自动创建)
aws ecs describe-task-definition --task-definition go-profile \
  --query 'taskDefinition' > /tmp/base.json
jq --arg img "$ECR_REPO:pr-$PR_NUMBER" --arg fam "go-profile-pr-$PR_NUMBER" \
   --arg db "profile_pr_$PR_NUMBER" \
  '.family=$fam | .containerDefinitions[0].image=$img
   | .containerDefinitions[0].environment |= map(
       if .name=="DATABASE_URL" then .value |= sub("/[^/]*$"; "/" + $db) else . end)
   | .runtimePlatform={"cpuArchitecture":"X86_64","operatingSystemFamily":"LINUX"}
   | del(.taskDefinitionArn,.revision,.status,.requiresAttributes,.compatibilities,.registeredAt,.registeredBy)' \
  /tmp/base.json > /tmp/taskdef.json
aws ecs register-task-definition --cli-input-json file:///tmp/taskdef.json \
  --query 'taskDefinition.family' --output text

# ── 3. 目标组 tg-pr-N(已存在则复用)──
TG_ARN=$(aws elbv2 describe-target-groups --names "tg-pr-$PR_NUMBER" \
  --query 'TargetGroups[0].TargetGroupArn' --output text 2>/dev/null || true)
if [ -z "$TG_ARN" ] || [ "$TG_ARN" = "None" ]; then
  TG_ARN=$(aws elbv2 create-target-group --name "tg-pr-$PR_NUMBER" \
    --protocol HTTP --port 8080 --vpc-id "$VPC_ID" --target-type ip \
    --health-check-path / \
    --query 'TargetGroups[0].TargetGroupArn' --output text)
fi
echo "target group: $TG_ARN"

# ── 4. ALB 分流规则:Host = pr-N.域名 → tg-pr-N(一个 ALB 服务所有 PR)──
RULE_ARN=$(aws elbv2 describe-rules --listener-arn "$LISTENER_ARN" \
  --query "Rules[?Conditions[0].Values[0]=='$HOST'].RuleArn | [0]" --output text)
if [ -z "$RULE_ARN" ] || [ "$RULE_ARN" = "None" ]; then
  aws elbv2 create-rule --listener-arn "$LISTENER_ARN" \
    --priority $((100 + PR_NUMBER)) \
    --conditions "Field=host-header,Values=$HOST" \
    --actions "Type=forward,TargetGroupArn=$TG_ARN" >/dev/null
fi
echo "listener rule for $HOST ready"

# ── 5. ECS 服务 app-pr-N(不存在则建,存在则滚动更新)──
STATUS=$(aws ecs describe-services --cluster "$CLUSTER" --services "app-pr-$PR_NUMBER" \
  --query 'services[0].status' --output text 2>/dev/null || true)
if [ "$STATUS" != "ACTIVE" ]; then
  aws ecs create-service --cluster "$CLUSTER" --service-name "app-pr-$PR_NUMBER" \
    --task-definition "go-profile-pr-$PR_NUMBER" --desired-count 1 \
    --launch-type FARGATE \
    --network-configuration "awsvpcConfiguration={subnets=[$SUBNETS],securityGroups=[$SG_ECS],assignPublicIp=DISABLED}" \
    --load-balancers "targetGroupArn=$TG_ARN,containerName=$CONTAINER_NAME,containerPort=8080" >/dev/null
else
  aws ecs update-service --cluster "$CLUSTER" --service "app-pr-$PR_NUMBER" \
    --task-definition "go-profile-pr-$PR_NUMBER" --force-new-deployment >/dev/null
fi
echo "ecs service app-pr-$PR_NUMBER deployed"

# ── 6. Cloudflare DNS:pr-N.域名 → ALB(没配域名时跳过)──
if [ -n "${CF_ZONE_ID:-}" ]; then
  EXISTING=$(curl -s -H "Authorization: Bearer $CF_API_TOKEN" \
    "https://api.cloudflare.com/client/v4/zones/$CF_ZONE_ID/dns_records?name=$HOST" \
    | jq -r '.result[0].id // empty')
  if [ -z "$EXISTING" ]; then
    curl -s -X POST -H "Authorization: Bearer $CF_API_TOKEN" -H "Content-Type: application/json" \
      "https://api.cloudflare.com/client/v4/zones/$CF_ZONE_ID/dns_records" \
      --data "{\"type\":\"CNAME\",\"name\":\"$HOST\",\"content\":\"$ALB_DNS\",\"proxied\":false,\"ttl\":60}" \
      | jq '.success'
  fi
  echo "======================================"
  echo "预览环境已就绪: http://$HOST"
  echo "======================================"
else
  echo "======================================"
  echo "未配置 Cloudflare(CF_ZONE_ID 为空),跳过 DNS 步骤"
  echo "临时访问方式(冒充域名的请求头):"
  echo "  curl -H 'Host: $HOST' http://$ALB_DNS/"
  echo "======================================"
fi
