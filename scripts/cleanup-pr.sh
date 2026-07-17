#!/bin/bash
# CDK 版清理:PR 合并/关闭后,`cdk destroy` 一条命令拆掉整栈
# ——这就是题目说的"这个 pr 被 closed 或者 merge 后就用 cdk 把这套环境删了"
set -euo pipefail

HOST="pr-${PR_NUMBER}.${DOMAIN}"

# 同样先换上部署角色
CREDS=$(aws sts assume-role --role-arn "$DEPLOY_ROLE_ARN" \
  --role-session-name "cleanup-pr-${PR_NUMBER}" \
  --query 'Credentials.[AccessKeyId,SecretAccessKey,SessionToken]' --output text)
export AWS_ACCESS_KEY_ID=$(echo "$CREDS" | awk '{print $1}')
export AWS_SECRET_ACCESS_KEY=$(echo "$CREDS" | awk '{print $2}')
export AWS_SESSION_TOKEN=$(echo "$CREDS" | awk '{print $3}')

# CDK 需要能合成栈才能销毁,给应用喂上占位值即可(销毁不看内容)
export DATABASE_URL="placeholder"
export GITHUB_TOKEN="placeholder"

cd cdk
cdk destroy "PrPreview-${PR_NUMBER}" --force
cd ..
echo "stack PrPreview-${PR_NUMBER} destroyed"

# 删 Cloudflare DNS 记录(没配域名时跳过)
if [ -n "${CF_ZONE_ID:-}" ]; then
  REC_ID=$(curl -s -H "Authorization: Bearer $CF_API_TOKEN" \
    "https://api.cloudflare.com/client/v4/zones/$CF_ZONE_ID/dns_records?name=$HOST" \
    | jq -r '.result[0].id // empty')
  [ -n "$REC_ID" ] && curl -s -X DELETE -H "Authorization: Bearer $CF_API_TOKEN" \
    "https://api.cloudflare.com/client/v4/zones/$CF_ZONE_ID/dns_records/$REC_ID" | jq '.success'
fi

echo "pr-$PR_NUMBER 预览环境已全部清理"
