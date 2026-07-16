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
    },
    {
      "Sid": "UseGitHubConnection",
      "Effect": "Allow",
      "Action": [
        "codeconnections:GetConnection",
        "codeconnections:GetConnectionToken",
        "codeconnections:UseConnection",
        "codestar-connections:GetConnection",
        "codestar-connections:GetConnectionToken",
        "codestar-connections:UseConnection"
      ],
      "Resource": [
        "arn:aws:codeconnections:us-east-1:688567307211:connection/88ffcccb-868e-4bdd-b73d-57c73593f9b0",
        "arn:aws:codestar-connections:us-east-1:688567307211:connection/88ffcccb-868e-4bdd-b73d-57c73593f9b0"
      ]
    }
  ]
}
```

> 踩坑记录:最后这条 UseGitHubConnection 是实战中补的——CodeBuild 通过
> GitHub App 连接(CodeConnections)拉代码和验证 webhook,这个动作用的是
> 项目的服务角色。漏掉它的现象:webhook 投递返回 400
> "Access denied to connection",构建根本不触发或 DOWNLOAD_SOURCE 失败。
