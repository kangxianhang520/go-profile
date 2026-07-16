#!/bin/bash
# PR 合并/关闭后,拆掉它的预览环境(与 deploy-pr.sh 逆序)
set -euo pipefail

HOST="pr-${PR_NUMBER}.${DOMAIN}"

# 同样先换上部署角色
CREDS=$(aws sts assume-role --role-arn "$DEPLOY_ROLE_ARN" \
  --role-session-name "cleanup-pr-${PR_NUMBER}" \
  --query 'Credentials.[AccessKeyId,SecretAccessKey,SessionToken]' --output text)
export AWS_ACCESS_KEY_ID=$(echo "$CREDS" | awk '{print $1}')
export AWS_SECRET_ACCESS_KEY=$(echo "$CREDS" | awk '{print $2}')
export AWS_SESSION_TOKEN=$(echo "$CREDS" | awk '{print $3}')

# 1. 删 ECS 服务
aws ecs update-service --cluster "$CLUSTER" --service "app-pr-$PR_NUMBER" \
  --desired-count 0 >/dev/null 2>&1 || true
aws ecs delete-service --cluster "$CLUSTER" --service "app-pr-$PR_NUMBER" \
  --force >/dev/null 2>&1 || true
echo "service deleted"

# 2. 删 ALB 分流规则
RULE_ARN=$(aws elbv2 describe-rules --listener-arn "$LISTENER_ARN" \
  --query "Rules[?Conditions[0].Values[0]=='$HOST'].RuleArn | [0]" --output text)
[ -n "$RULE_ARN" ] && [ "$RULE_ARN" != "None" ] && aws elbv2 delete-rule --rule-arn "$RULE_ARN"
echo "listener rule deleted"

# 3. 删目标组
TG_ARN=$(aws elbv2 describe-target-groups --names "tg-pr-$PR_NUMBER" \
  --query 'TargetGroups[0].TargetGroupArn' --output text 2>/dev/null || true)
[ -n "$TG_ARN" ] && [ "$TG_ARN" != "None" ] && aws elbv2 delete-target-group --target-group-arn "$TG_ARN"
echo "target group deleted"

# 4. 删 Cloudflare DNS 记录
REC_ID=$(curl -s -H "Authorization: Bearer $CF_API_TOKEN" \
  "https://api.cloudflare.com/client/v4/zones/$CF_ZONE_ID/dns_records?name=$HOST" \
  | jq -r '.result[0].id // empty')
[ -n "$REC_ID" ] && curl -s -X DELETE -H "Authorization: Bearer $CF_API_TOKEN" \
  "https://api.cloudflare.com/client/v4/zones/$CF_ZONE_ID/dns_records/$REC_ID" | jq '.success'

echo "pr-$PR_NUMBER 预览环境已清理完毕"
