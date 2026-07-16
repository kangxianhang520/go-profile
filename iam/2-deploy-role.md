# 角色 2:部署角色 pr-deploy-role(构建时被 AssumeRole)

真正有权创建/删除环境的角色。**只允许"构建角色"变身成它**,别人不行。

- 创建位置:IAM → 角色 → 创建角色 → 可信实体选 **自定义信任策略**,粘贴:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": { "AWS": "arn:aws:iam::688567307211:role/pr-build-role" },
      "Action": "sts:AssumeRole"
    }
  ]
}
```

- 附加内联策略 `pr-deploy-policy`:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "ManagePreviewServices",
      "Effect": "Allow",
      "Action": [
        "ecs:RegisterTaskDefinition",
        "ecs:DescribeTaskDefinition",
        "ecs:CreateService",
        "ecs:UpdateService",
        "ecs:DeleteService",
        "ecs:DescribeServices"
      ],
      "Resource": "*"
    },
    {
      "Sid": "ManageAlbRouting",
      "Effect": "Allow",
      "Action": [
        "elasticloadbalancing:CreateTargetGroup",
        "elasticloadbalancing:DeleteTargetGroup",
        "elasticloadbalancing:DescribeTargetGroups",
        "elasticloadbalancing:CreateRule",
        "elasticloadbalancing:DeleteRule",
        "elasticloadbalancing:DescribeRules"
      ],
      "Resource": "*"
    },
    {
      "Sid": "PassTaskRolesToEcs",
      "Effect": "Allow",
      "Action": "iam:PassRole",
      "Resource": "arn:aws:iam::688567307211:role/ecsTaskExecutionRole*",
      "Condition": { "StringEquals": { "iam:PassedToService": "ecs-tasks.amazonaws.com" } }
    }
  ]
}
```

> `iam:PassRole` 那条的意思:部署角色在建 ECS 服务时,要把"运行时角色"递给容器,
> 这个"递交"动作本身也需要授权——这是三角色链条的连接点。
> 如果你的任务定义用的执行角色名字不是 ecsTaskExecutionRole 开头,把 Resource 改成实际 ARN
> (查看方法:ECS → 任务定义 → go-profile → 任务执行角色)。
