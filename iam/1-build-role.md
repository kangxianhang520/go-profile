# 角色 1:构建角色 pr-build-role(给 CodeBuild 用)

只能干三件事:推镜像、写日志、"变身"成部署角色。**它自己不能碰 ECS/ALB/DNS。**

- 创建位置:IAM → 角色 → 创建角色 → 可信实体选 **AWS 服务 → CodeBuild**
- 附加下面这个内联策略(名字随意,如 `pr-build-policy`):

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "PushImageToECR",
      "Effect": "Allow",
      "Action": [
        "ecr:GetAuthorizationToken",
        "ecr:BatchCheckLayerAvailability",
        "ecr:InitiateLayerUpload",
        "ecr:UploadLayerPart",
        "ecr:CompleteLayerUpload",
        "ecr:PutImage",
        "ecr:BatchGetImage",
        "ecr:GetDownloadUrlForLayer"
      ],
      "Resource": "*"
    },
    {
      "Sid": "WriteLogs",
      "Effect": "Allow",
      "Action": ["logs:CreateLogGroup", "logs:CreateLogStream", "logs:PutLogEvents"],
      "Resource": "*"
    },
    {
      "Sid": "ReadCloudflareToken",
      "Effect": "Allow",
      "Action": ["ssm:GetParameters", "ssm:GetParameter"],
      "Resource": "arn:aws:ssm:us-east-1:688567307211:parameter/homework/*"
    },
    {
      "Sid": "BecomeDeployRole",
      "Effect": "Allow",
      "Action": "sts:AssumeRole",
      "Resource": "arn:aws:iam::688567307211:role/pr-deploy-role"
    }
  ]
}
```
