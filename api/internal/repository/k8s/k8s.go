package k8s

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/jsii-runtime-go"
	"github.com/cdk8s-team/cdk8s-core-go/cdk8s/v2"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

type Bot struct {
	BotID         string
	GitRepoURL    string
	GitEntrypoint string
	MinReplicas   float64
	MaxReplicas   float64
}

type K8SOperator struct {
	dynClient       dynamic.Interface
	discoveryClient discovery.DiscoveryInterface

	RunnerImage       string
	SidecarImage      string
	PrometheusAddress string
	KafkaBrokers      string
	Namespace         string
	RequestsCPU       float64
	LimitsCPU         float64
	RequestsMemory    float64
	LimitsMemory      float64

	Logger *zap.Logger
}

func (o *K8SOperator) sugar() *zap.SugaredLogger {
	if o != nil && o.Logger != nil {
		return o.Logger.Sugar()
	}
	return zap.NewNop().Sugar()
}

func NewK8SOperator(config *rest.Config, logger *zap.Logger) (*K8SOperator, error) {
	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	disc, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}

	return &K8SOperator{
		dynClient:       dyn,
		discoveryClient: disc,
		Logger:          logger,
	}, nil
}

func (o *K8SOperator) applyYaml(ctx context.Context, rawYaml string) error {
	gr, err := restmapper.GetAPIGroupResources(o.discoveryClient)
	if err != nil {
		return fmt.Errorf("failed to get api group resources: %w", err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(gr)

	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(rawYaml), 4096)

	for {
		var obj unstructured.Unstructured
		if err := decoder.Decode(&obj); err != nil {
			break
		}
		if obj.Object == nil {
			continue
		}

		gvk := obj.GroupVersionKind()
		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return fmt.Errorf("failed to get rest mapping for %s: %w", gvk.Kind, err)
		}

		var dr dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			dr = o.dynClient.Resource(mapping.Resource).Namespace(obj.GetNamespace())
		} else {
			dr = o.dynClient.Resource(mapping.Resource)
		}

		data, err := obj.MarshalJSON()
		if err != nil {
			return fmt.Errorf("failed to marshal object to json: %w", err)
		}

		_, err = dr.Patch(ctx, obj.GetName(), types.ApplyPatchType, data, metav1.PatchOptions{
			FieldManager: "bots-operator-service",
			Force:        jsii.Bool(true),
		})

		if err != nil {
			return fmt.Errorf("failed to apply resource %s/%s: %w", gvk.Kind, obj.GetName(), err)
		}

		o.sugar().Infof("Applied %s: %s", gvk.Kind, obj.GetName())
	}

	return nil
}

func (o *K8SOperator) DeployBot(ctx context.Context, bot Bot) error {
	app := cdk8s.NewApp(nil)

	// Собираем props
	props := &BotProps{
		// Уникальные данные бота
		BotID:         bot.BotID,
		GitRepoURL:    bot.GitRepoURL,
		GitEntrypoint: bot.GitEntrypoint,

		// Общие данные из структуры оператора
		Namespace:         o.Namespace,
		RunnerImage:       o.RunnerImage,
		SidecarImage:      o.SidecarImage,
		PrometheusAddress: o.PrometheusAddress,
		KafkaBrokers:      o.KafkaBrokers,

		// Можно также добавить дефолты, если их нет в Bot
		MinReplicas:      bot.MinReplicas,
		MaxReplicas:      bot.MaxReplicas,
		ScalingThreshold: "100",

		// Ссылки на секреты (обычно они тоже стандартные в одном namespace)
		KafkaAuthRef: SecretKeyRef{Name: "kafka-auth", Key: "password"},
		GitTokenRef:  SecretKeyRef{Name: "git-tokens", Key: bot.BotID},

		RequestsCPU:    o.RequestsCPU,
		LimitsCPU:      o.LimitsCPU,
		RequestsMemory: o.RequestsMemory,
		LimitsMemory:   o.LimitsMemory,
	}

	chartId := fmt.Sprintf("bots-%s", bot.BotID)
	rawYaml := NewBotChart(app, chartId, props)

	return o.applyYaml(ctx, rawYaml)
}
