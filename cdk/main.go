// CDK 应用:为一个 PR 部署/销毁一整套独立预览环境(题目要求的 CDK 版)。
// 每套环境 = 任务定义 + Fargate 服务 + 目标组 + ALB 分流规则 + 专属 Lambda。
// PR 合并/关闭时 `cdk destroy` 一条命令整栈拆除——这就是"用 CDK 把这套环境删了"。
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	elbv2 "github.com/aws/aws-cdk-go/awscdk/v2/awselasticloadbalancingv2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/jsii-runtime-go"
)

// env 读取必填环境变量,缺了直接报错(由 buildspec 注入)
func env(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("missing env %s", key))
	}
	return v
}

// 每个 PR 专属 Lambda 的转发代码:带 Host 头访问 ALB,落到本 PR 的容器上
const forwarderJS = `exports.handler = async (event) => {
  const path = (event && event.rawPath) || "/";
  const method = (event && event.requestContext && event.requestContext.http && event.requestContext.http.method) || "GET";
  const resp = await fetch("http://" + process.env.ALB_DNS + path, { method, headers: { host: process.env.PR_HOST } });
  return { statusCode: resp.status, headers: { "content-type": resp.headers.get("content-type") }, body: await resp.text() };
};`

func main() {
	defer jsii.Close()

	pr := env("PR_NUMBER")
	prNum, err := strconv.Atoi(pr)
	if err != nil {
		panic("PR_NUMBER must be a number: " + pr)
	}
	host := fmt.Sprintf("pr-%s.%s", pr, env("DOMAIN"))

	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("PrPreview-"+pr), &awscdk.StackProps{
		Env: &awscdk.Environment{
			Account: jsii.String(env("CDK_DEFAULT_ACCOUNT")),
			Region:  jsii.String(env("CDK_DEFAULT_REGION")),
		},
	})

	// ── 导入既有资源(共享底座,不由本栈创建/销毁)──
	subnetIDs := strings.Split(env("SUBNETS"), ",")
	azs := strings.Split(env("AZS"), ",")
	vpc := awsec2.Vpc_FromVpcAttributes(stack, jsii.String("Vpc"), &awsec2.VpcAttributes{
		VpcId:             jsii.String(env("VPC_ID")),
		AvailabilityZones: jsii.Strings(azs...),
		PrivateSubnetIds:  jsii.Strings(subnetIDs...),
	})
	cluster := awsecs.Cluster_FromClusterAttributes(stack, jsii.String("Cluster"), &awsecs.ClusterAttributes{
		ClusterName: jsii.String(env("CLUSTER")),
		Vpc:         vpc,
	})
	sgEcs := awsec2.SecurityGroup_FromSecurityGroupId(stack, jsii.String("SgEcs"), jsii.String(env("SG_ECS")), nil)
	sgAlb := awsec2.SecurityGroup_FromSecurityGroupId(stack, jsii.String("SgAlb"), jsii.String(env("SG_ALB")), nil)
	sgLambda := awsec2.SecurityGroup_FromSecurityGroupId(stack, jsii.String("SgLambda"), jsii.String(env("SG_LAMBDA")), nil)
	execRole := awsiam.Role_FromRoleArn(stack, jsii.String("ExecRole"), jsii.String(env("EXEC_ROLE_ARN")), &awsiam.FromRoleArnOptions{Mutable: jsii.Bool(false)})
	lambdaRole := awsiam.Role_FromRoleArn(stack, jsii.String("FnRole"), jsii.String(env("LAMBDA_ROLE_ARN")), &awsiam.FromRoleArnOptions{Mutable: jsii.Bool(false)})
	logGroup := awslogs.LogGroup_FromLogGroupName(stack, jsii.String("Lg"), jsii.String("/ecs/go-profile"))
	listener := elbv2.ApplicationListener_FromApplicationListenerAttributes(stack, jsii.String("Listener"), &elbv2.ApplicationListenerAttributes{
		ListenerArn:   jsii.String(env("LISTENER_ARN")),
		SecurityGroup: sgAlb,
	})
	privateSubnets := &awsec2.SubnetSelection{Subnets: vpc.PrivateSubnets()}

	// ── 1. 任务定义:pr-N 镜像 + 本 PR 独立数据库 ──
	taskDef := awsecs.NewFargateTaskDefinition(stack, jsii.String("TaskDef"), &awsecs.FargateTaskDefinitionProps{
		Family:        jsii.String("go-profile-pr-" + pr),
		Cpu:           jsii.Number(256),
		MemoryLimitMiB: jsii.Number(512),
		ExecutionRole: execRole,
		RuntimePlatform: &awsecs.RuntimePlatform{
			CpuArchitecture:       awsecs.CpuArchitecture_X86_64(),
			OperatingSystemFamily: awsecs.OperatingSystemFamily_LINUX(),
		},
	})
	taskDef.AddContainer(jsii.String("app"), &awsecs.ContainerDefinitionOptions{
		Image: awsecs.ContainerImage_FromRegistry(jsii.String(env("ECR_REPO")+":pr-"+pr), nil),
		PortMappings: &[]*awsecs.PortMapping{{ContainerPort: jsii.Number(8080)}},
		Environment: &map[string]*string{
			"DATABASE_URL": jsii.String(env("DATABASE_URL")),
			"GITHUB_TOKEN": jsii.String(env("GITHUB_TOKEN")),
		},
		Logging: awsecs.LogDrivers_AwsLogs(&awsecs.AwsLogDriverProps{
			StreamPrefix: jsii.String("pr-" + pr),
			LogGroup:     logGroup,
		}),
	})

	// ── 2. 目标组 + ALB 按域名分流规则 ──
	tg := elbv2.NewApplicationTargetGroup(stack, jsii.String("Tg"), &elbv2.ApplicationTargetGroupProps{
		TargetGroupName: jsii.String("tg-pr-" + pr),
		Vpc:             vpc,
		Port:            jsii.Number(8080),
		Protocol:        elbv2.ApplicationProtocol_HTTP,
		TargetType:      elbv2.TargetType_IP,
		HealthCheck:     &elbv2.HealthCheck{Path: jsii.String("/")},
	})
	elbv2.NewApplicationListenerRule(stack, jsii.String("Rule"), &elbv2.ApplicationListenerRuleProps{
		Listener: listener,
		Priority: jsii.Number(float64(100 + prNum)),
		Conditions: &[]elbv2.ListenerCondition{
			elbv2.ListenerCondition_HostHeaders(jsii.Strings(host)),
		},
		TargetGroups: &[]elbv2.IApplicationTargetGroup{tg},
	})

	// ── 3. Fargate 服务 ──
	svc := awsecs.NewFargateService(stack, jsii.String("Svc"), &awsecs.FargateServiceProps{
		ServiceName:    jsii.String("app-pr-" + pr),
		Cluster:        cluster,
		TaskDefinition: taskDef,
		DesiredCount:   jsii.Number(1),
		SecurityGroups: &[]awsec2.ISecurityGroup{sgEcs},
		VpcSubnets:     privateSubnets,
		AssignPublicIp: jsii.Bool(false),
		// 起不来就快速失败并回滚,预览环境不值得干等
		CircuitBreaker:    &awsecs.DeploymentCircuitBreaker{Rollback: jsii.Bool(true)},
		MinHealthyPercent: jsii.Number(0),
	})
	svc.AttachToApplicationTargetGroup(tg)

	// ── 4. 本 PR 专属 Lambda(题目:"连接多个 lambda")──
	awslambda.NewFunction(stack, jsii.String("Fn"), &awslambda.FunctionProps{
		FunctionName:   jsii.String("forwarder-pr-" + pr),
		Runtime:        awslambda.Runtime_NODEJS_20_X(),
		Handler:        jsii.String("index.handler"),
		Code:           awslambda.Code_FromInline(jsii.String(forwarderJS)),
		Timeout:        awscdk.Duration_Seconds(jsii.Number(25)),
		Vpc:            vpc,
		VpcSubnets:     privateSubnets,
		SecurityGroups: &[]awsec2.ISecurityGroup{sgLambda},
		Role:           lambdaRole,
		Environment: &map[string]*string{
			"ALB_DNS": jsii.String(env("ALB_DNS")),
			"PR_HOST": jsii.String(host),
		},
	})

	awscdk.NewCfnOutput(stack, jsii.String("PreviewURL"), &awscdk.CfnOutputProps{
		Value: jsii.String("http://" + host),
	})

	app.Synth(nil)
}
