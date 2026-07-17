#!/bin/bash
# CDK 版:为一个 PR 部署整套预览环境(栈 PrPreview-N)
# 栈内容 = 任务定义(独立数据库) + Fargate 服务 + 目标组 + ALB 分流规则 + 专属 Lambda
set -euo pipefail

HOST="pr-${PR_NUMBER}.${DOMAIN}"

# ── 1. 换上"部署角色"(考点:构建角色只能推镜像,部署必须 AssumeRole)──
CREDS=$(aws sts assume-role --role-arn "$DEPLOY_ROLE_ARN" \
  --role-session-name "deploy-pr-${PR_NUMBER}" \
  --query 'Credentials.[AccessKeyId,SecretAccessKey,SessionToken]' --output text)
export AWS_ACCESS_KEY_ID=$(echo "$CREDS" | awk '{print $1}')
export AWS_SECRET_ACCESS_KEY=$(echo "$CREDS" | awk '{print $2}')
export AWS_SESSION_TOKEN=$(echo "$CREDS" | awk '{print $3}')
echo "assumed deploy role"

# ── 2. 从线上任务定义取数据库连接串和 token,拼出本 PR 的独立库连接串 ──
BASE_DB=$(aws ecs describe-task-definition --task-definition go-profile \
  --query "taskDefinition.containerDefinitions[0].environment[?name=='DATABASE_URL'].value" --output text)
export DATABASE_URL=$(echo "$BASE_DB" | sed -E "s|/[^/]*$|/profile_pr_${PR_NUMBER}|")
export GITHUB_TOKEN=$(aws ecs describe-task-definition --task-definition go-profile \
  --query "taskDefinition.containerDefinitions[0].environment[?name=='GITHUB_TOKEN'].value" --output text)

# ── 3. CDK 一条命令部署整栈 ──
cd cdk
cdk deploy "PrPreview-${PR_NUMBER}" --require-approval never
cd ..

# ── 4. Cloudflare DNS:pr-N.域名 → ALB(没配域名时跳过)──
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
fi

echo "======================================"
echo "预览环境已就绪: http://$HOST"
echo "专属 Lambda: forwarder-pr-$PR_NUMBER"
echo "======================================"
