package k8s

import (
	"fmt"

	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
	"github.com/cdk8s-team/cdk8s-core-go/cdk8s/v2"
	"github.com/cdk8s-team/cdk8s-plus-go/cdk8splus27/v2"
)

// =============================================================================
// PROPS
// =============================================================================

type SecretKeyRef struct {
	Name string
	Key  string
}

type BotProps struct {
	Namespace string
	BotID     string

	RunnerImage  string
	SidecarImage string

	MinReplicas       float64
	MaxReplicas       float64
	PrometheusAddress string
	ScalingThreshold  string

	KafkaBrokers  string
	TopicRegex    string
	ConsumerGroup string
	KafkaAuthRef  SecretKeyRef

	GitRepoURL    string
	GitEntrypoint string
	GitTokenRef   SecretKeyRef

	RequestsCPU    float64
	LimitsCPU      float64
	RequestsMemory float64
	LimitsMemory   float64
}

// =============================================================================
// CHART GENERATOR
// =============================================================================

func NewBotChart(scope constructs.Construct, id string, props *BotProps) string {
	chart := cdk8s.NewChart(scope, jsii.String(id), &cdk8s.ChartProps{
		Namespace: jsii.String(props.Namespace),
	})

	gitSecret := cdk8splus27.Secret_FromSecretName(
		chart,
		jsii.String("git-sec"),
		jsii.String(props.GitTokenRef.Name),
	)
	kafkaSecret := cdk8splus27.Secret_FromSecretName(
		chart,
		jsii.String("kafka-sec"),
		jsii.String(props.KafkaAuthRef.Name),
	)

	codeVol := cdk8splus27.Volume_FromEmptyDir(
		chart,
		jsii.String("code-vol-id"),
		jsii.String("code-volume"),
		&cdk8splus27.EmptyDirVolumeOptions{
			Medium:    cdk8splus27.EmptyDirMedium_MEMORY,
			SizeLimit: cdk8s.Size_Mebibytes(jsii.Number(128)),
		},
	)

	tmpVol := cdk8splus27.Volume_FromEmptyDir(
		chart,
		jsii.String("tmp-vol-id"),
		jsii.String("tmp-volume"),
		&cdk8splus27.EmptyDirVolumeOptions{
			Medium:    cdk8splus27.EmptyDirMedium_MEMORY,
			SizeLimit: cdk8s.Size_Mebibytes(jsii.Number(64)),
		},
	)

	dep := cdk8splus27.NewDeployment(chart, jsii.String("dep"), &cdk8splus27.DeploymentProps{
		Metadata: &cdk8s.ApiObjectMetadata{
			Name: jsii.String(fmt.Sprintf("bots-%s", props.BotID)),
			Labels: &map[string]*string{
				"app":    jsii.String("user-bots"),
				"bot_id": jsii.String(props.BotID),
			},
		},
		Replicas: jsii.Number(props.MinReplicas),
	})

	cdk8s.ApiObject_Of(dep).AddJsonPatch(cdk8s.JsonPatch_Add(
		jsii.String("/spec/template/spec/shareProcessNamespace"),
		jsii.Bool(false),
	))

	init := dep.AddInitContainer(&cdk8splus27.ContainerProps{
		Name:  jsii.String("git-cloner"),
		Image: jsii.String("alpine/git:latest"),

		EnvVariables: &map[string]cdk8splus27.EnvValue{
			"REPO_URL": cdk8splus27.EnvValue_FromValue(jsii.String(props.GitRepoURL)),
			"GIT_TOKEN": cdk8splus27.EnvValue_FromSecretValue(&cdk8splus27.SecretValue{
				Secret: gitSecret,
				Key:    jsii.String(props.GitTokenRef.Key),
			}, nil),
		},

		Command: jsii.Strings("sh", "-c"),
		Args: jsii.Strings(`
			if [ -n "$GIT_TOKEN" ]; then
				FULL_URL=$(echo $REPO_URL | sed "s/https:\/\//https:\/\/$GIT_TOKEN@/")
			else
				FULL_URL=$REPO_URL
			fi
			echo "Cloning $REPO_URL..."
			git clone --depth 1 $FULL_URL /app/code

			сhown -R 2000:2000 /app/code
		`),
		Resources: &cdk8splus27.ContainerResources{
			Cpu:    &cdk8splus27.CpuResources{Limit: cdk8splus27.Cpu_Millis(jsii.Number(500))},
			Memory: &cdk8splus27.MemoryResources{Limit: cdk8s.Size_Mebibytes(jsii.Number(128))},
		},
	})
	init.Mount(jsii.String("/app/code"), codeVol, nil)

	dep.AddContainer(&cdk8splus27.ContainerProps{
		Name:  jsii.String("sidecar"),
		Image: jsii.String(props.SidecarImage),
		Port:  jsii.Number(8081),

		EnvVariables: &map[string]cdk8splus27.EnvValue{
			"HTTP_PORT":     cdk8splus27.EnvValue_FromValue(jsii.String("8081")),
			"KAFKA_BROKERS": cdk8splus27.EnvValue_FromValue(jsii.String(props.KafkaBrokers)),
			"GROUP_ID":      cdk8splus27.EnvValue_FromValue(jsii.String(props.ConsumerGroup)),
			"TOPIC_REGEX":   cdk8splus27.EnvValue_FromValue(jsii.String(props.TopicRegex)),
			"KAFKA_USER":    cdk8splus27.EnvValue_FromValue(jsii.String("admin")),

			"KAFKA_PASSWORD": cdk8splus27.EnvValue_FromSecretValue(&cdk8splus27.SecretValue{
				Secret: kafkaSecret,
				Key:    jsii.String(props.KafkaAuthRef.Key),
			}, nil),
		},

		Resources: &cdk8splus27.ContainerResources{
			Cpu: &cdk8splus27.CpuResources{
				Request: cdk8splus27.Cpu_Millis(jsii.Number(50)),
				Limit:   cdk8splus27.Cpu_Millis(jsii.Number(200)),
			},
			Memory: &cdk8splus27.MemoryResources{
				Limit: cdk8s.Size_Mebibytes(jsii.Number(64)),
			},
		},

		Readiness: cdk8splus27.Probe_FromHttpGet(jsii.String("/health/ready"), &cdk8splus27.HttpGetProbeOptions{
			Port:                jsii.Number(8081),
			InitialDelaySeconds: cdk8s.Duration_Seconds(jsii.Number(2)),
		}),
	})

	entrypoint := "main.py"
	if props.GitEntrypoint != "" {
		entrypoint = props.GitEntrypoint
	}

	runner := dep.AddContainer(&cdk8splus27.ContainerProps{
		Name:       jsii.String("app"),
		Image:      jsii.String(props.RunnerImage),
		WorkingDir: jsii.String("/app/code"),
		Command:    jsii.Strings("python", "-u", entrypoint),

		EnvVariables: &map[string]cdk8splus27.EnvValue{
			"TELEGRAM_API_URL": cdk8splus27.EnvValue_FromValue(jsii.String("http://localhost:8081")),
			"HTTP_PROXY":       cdk8splus27.EnvValue_FromValue(jsii.String("http://localhost:8081")),
			"HTTPS_PROXY":      cdk8splus27.EnvValue_FromValue(jsii.String("http://localhost:8081")),

			"BOT_TOKEN": cdk8splus27.EnvValue_FromSecretValue(&cdk8splus27.SecretValue{
				Secret: gitSecret,
				Key:    jsii.String("BOT_TOKEN"),
			}, nil),
		},

		Resources: &cdk8splus27.ContainerResources{
			Cpu: &cdk8splus27.CpuResources{
				Request: cdk8splus27.Cpu_Millis(jsii.Number(props.RequestsCPU)),
				Limit:   cdk8splus27.Cpu_Millis(jsii.Number(props.LimitsCPU)),
			},
			Memory: &cdk8splus27.MemoryResources{
				Request: cdk8s.Size_Mebibytes(jsii.Number(props.RequestsMemory)),
				Limit:   cdk8s.Size_Mebibytes(jsii.Number(props.LimitsMemory)),
			},
		},

		SecurityContext: &cdk8splus27.ContainerSecurityContextProps{
			User:                     jsii.Number(2000), // RunAsUser
			Group:                    jsii.Number(2000), // RunAsGroup
			EnsureNonRoot:            jsii.Bool(true),   // RunAsNonRoot
			ReadOnlyRootFilesystem:   jsii.Bool(true),
			AllowPrivilegeEscalation: jsii.Bool(false),
		},
	})

	runner.Mount(jsii.String("/app/code"), codeVol, nil)
	runner.Mount(jsii.String("/tmp"), tmpVol, nil)

	promQuery := fmt.Sprintf(
		`sum(kafka_consumergroup_lag{consumergroup="%s", topic=~"%s"})`,
		props.ConsumerGroup,
		props.TopicRegex,
	)

	kedaAuthName := fmt.Sprintf("keda-kafka-auth-%s", props.BotID)

	auth := cdk8s.NewApiObject(chart, jsii.String("keda-auth"), &cdk8s.ApiObjectProps{
		ApiVersion: jsii.String("keda.sh/v1alpha1"),
		Kind:       jsii.String("TriggerAuthentication"),
		Metadata:   &cdk8s.ApiObjectMetadata{Name: jsii.String(kedaAuthName)},
	})

	auth.AddJsonPatch(cdk8s.JsonPatch_Add(
		jsii.String("/spec"),
		map[string]interface{}{
			"secretTargetRef": []map[string]string{
				{
					"parameter": "password",
					"name":      props.KafkaAuthRef.Name,
					"key":       props.KafkaAuthRef.Key,
				},
				{
					"parameter": "username",
					"name":      props.KafkaAuthRef.Name,
					"key":       "username",
				},
			},
		},
	))

	scaler := cdk8s.NewApiObject(chart, jsii.String("scaler"), &cdk8s.ApiObjectProps{
		ApiVersion: jsii.String("keda.sh/v1alpha1"),
		Kind:       jsii.String("ScaledObject"),
		Metadata: &cdk8s.ApiObjectMetadata{
			Name: jsii.String(fmt.Sprintf("scaler-%s", props.BotID)),
		},
	})

	scaler.AddJsonPatch(cdk8s.JsonPatch_Add(
		jsii.String("/spec"),
		map[string]interface{}{
			"scaleTargetRef": map[string]interface{}{
				"name": *dep.Name(),
			},
			"minReplicaCount": props.MinReplicas,
			"maxReplicaCount": props.MaxReplicas,
			"pollingInterval": 15,
			"cooldownPeriod":  30,
			"triggers": []interface{}{
				map[string]interface{}{
					"type": "prometheus",
					"metadata": map[string]string{
						"serverAddress": props.PrometheusAddress,
						"query":         promQuery,
						"threshold":     props.ScalingThreshold,
					},
				},
			},
		},
	))

	return *cdk8s.Yaml_Stringify(chart)
}
