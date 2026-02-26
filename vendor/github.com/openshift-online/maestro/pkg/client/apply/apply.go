// Package apply provides utilities to apply Kubernetes manifests to Maestro as ManifestWork.
package apply

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/pkg/client/cloudevents/grpcsource"
	"github.com/openshift-online/ocm-sdk-go/logging"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"

	workv1client "open-cluster-management.io/api/client/work/clientset/versioned/typed/work/v1"
	workv1 "open-cluster-management.io/api/work/v1"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/generic/options/grpc"
)

// Options configures how manifests are applied to Maestro.
type Options struct {
	MaestroServer        string
	GRPCServer           string
	ConsumerName         string
	WorkName             string
	SourceID             string
	InsecureSkipVerify   bool
	ServerHealthTimeout time.Duration
}

// WorkInfo holds basic info about a ManifestWork for listing.
type WorkInfo struct {
	Name        string
	ClusterName string // derived from work name (strips "-work" suffix)
	Consumer    string // consumer name (empty when listing single consumer)
	Created     string
}

// ListManifestWorks lists ManifestWorks for the given Maestro consumer.
// When opts.ConsumerName is empty, lists from all consumers.
func ListManifestWorks(ctx context.Context, opts Options) ([]WorkInfo, error) {
	if opts.ConsumerName != "" {
		return listManifestWorksForConsumer(ctx, opts, opts.ConsumerName)
	}
	return listManifestWorksFromAllConsumers(ctx, opts)
}

func listManifestWorksForConsumer(ctx context.Context, opts Options, consumerName string) ([]WorkInfo, error) {
	workClient, err := newWorkClient(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("create work client: %w", err)
	}

	workList, err := workClient.ManifestWorks(consumerName).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list works for consumer %s: %w", consumerName, err)
	}

	result := make([]WorkInfo, 0, len(workList.Items))
	for _, w := range workList.Items {
		clusterName := w.Name
		if strings.HasSuffix(w.Name, "-work") {
			clusterName = strings.TrimSuffix(w.Name, "-work")
		}
		result = append(result, WorkInfo{
			Name:        w.Name,
			ClusterName: clusterName,
			Consumer:    consumerName,
			Created:     w.CreationTimestamp.Format("2006-01-02 15:04:05"),
		})
	}
	return result, nil
}

func listManifestWorksFromAllConsumers(ctx context.Context, opts Options) ([]WorkInfo, error) {
	apiClient := newAPIClient(opts)
	consumerList, _, err := apiClient.DefaultAPI.ApiMaestroV1ConsumersGet(ctx).Size(1000).Execute()
	if err != nil {
		return nil, fmt.Errorf("list consumers: %w", err)
	}

	workClient, err := newWorkClient(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("create work client: %w", err)
	}

	var result []WorkInfo
	for _, c := range consumerList.GetItems() {
		consumerName := c.GetName()
		if consumerName == "" {
			consumerName = c.GetId()
		}
		if consumerName == "" {
			continue
		}

		workList, err := workClient.ManifestWorks(consumerName).List(ctx, metav1.ListOptions{})
		if err != nil {
			// Skip consumers that fail (e.g. no works for this source)
			continue
		}

		for _, w := range workList.Items {
			clusterName := w.Name
			if strings.HasSuffix(w.Name, "-work") {
				clusterName = strings.TrimSuffix(w.Name, "-work")
			}
			result = append(result, WorkInfo{
				Name:        w.Name,
				ClusterName: clusterName,
				Consumer:    consumerName,
				Created:     w.CreationTimestamp.Format("2006-01-02 15:04:05"),
			})
		}
	}
	return result, nil
}

func newAPIClient(opts Options) *openapi.APIClient {
	return openapi.NewAPIClient(&openapi.Configuration{
		DefaultHeader: make(map[string]string),
		UserAgent:     "maestro-apply/1.0",
		Debug:         false,
		Servers:       openapi.ServerConfigurations{{URL: opts.MaestroServer, Description: "maestro"}},
		OperationServers: map[string]openapi.ServerConfigurations{},
		HTTPClient: &http.Client{
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: opts.InsecureSkipVerify}},
			Timeout:   10 * time.Second,
		},
	})
}

// GetManifestWork fetches a ManifestWork from Maestro for the given consumer.
// WorkName is the ManifestWork name (e.g. "{cluster-name}-work").
func GetManifestWork(ctx context.Context, opts Options) (*workv1.ManifestWork, error) {
	if opts.ConsumerName == "" {
		return nil, fmt.Errorf("consumer name is required")
	}
	if opts.WorkName == "" {
		return nil, fmt.Errorf("work name is required")
	}

	workClient, err := newWorkClient(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("create work client: %w", err)
	}

	mw, err := workClient.ManifestWorks(opts.ConsumerName).Get(ctx, opts.WorkName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("manifest work %q not found for consumer %q", opts.WorkName, opts.ConsumerName)
		}
		return nil, fmt.Errorf("get work: %w", err)
	}
	return mw, nil
}

// DeleteManifestWork deletes a ManifestWork from Maestro for the given consumer.
// WorkName is the ManifestWork name (e.g. "{cluster-name}-work").
func DeleteManifestWork(ctx context.Context, opts Options) error {
	if opts.ConsumerName == "" {
		return fmt.Errorf("consumer name is required")
	}
	if opts.WorkName == "" {
		return fmt.Errorf("work name is required")
	}

	workClient, err := newWorkClient(ctx, opts)
	if err != nil {
		return fmt.Errorf("create work client: %w", err)
	}

	err = workClient.ManifestWorks(opts.ConsumerName).Delete(ctx, opts.WorkName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete work: %w", err)
	}
	return nil
}

// ApplyManifestsFromFile reads a YAML file (single or multi-resource with ---),
// wraps it in a ManifestWork, and applies it to the given Maestro consumer.
func ApplyManifestsFromFile(ctx context.Context, filePath string, opts Options) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	return ApplyManifests(ctx, data, opts)
}

// ApplyManifests reads YAML (single or multi-resource with ---),
// wraps it in a ManifestWork, and applies it to the given Maestro consumer.
func ApplyManifests(ctx context.Context, yamlContent []byte, opts Options) error {
	objs, err := decodeYAML(yamlContent)
	if err != nil {
		return fmt.Errorf("decode YAML: %w", err)
	}
	if len(objs) == 0 {
		return fmt.Errorf("no resources found in YAML")
	}

	workName := opts.WorkName
	if workName == "" {
		workName = objs[0].obj.GetName() + "-work"
	}

	mw, err := buildManifestWork(objs, workName)
	if err != nil {
		return fmt.Errorf("build ManifestWork: %w", err)
	}

	workClient, err := newWorkClient(ctx, opts)
	if err != nil {
		return fmt.Errorf("create work client: %w", err)
	}

	consumerName := opts.ConsumerName
	if consumerName == "" {
		return fmt.Errorf("consumer name is required")
	}

	existing, err := workClient.ManifestWorks(consumerName).Get(ctx, mw.Name, metav1.GetOptions{})
	if err == nil {
		patchData, err := grpcsource.ToWorkPatch(existing, mw)
		if err != nil {
			return fmt.Errorf("build patch: %w", err)
		}
		_, err = workClient.ManifestWorks(consumerName).Patch(ctx, mw.Name, types.MergePatchType, patchData, metav1.PatchOptions{})
		if err != nil {
			return fmt.Errorf("patch work: %w", err)
		}
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get work: %w", err)
	}

	created, err := workClient.ManifestWorks(consumerName).Create(ctx, mw, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create work: %w", err)
	}
	_ = created
	return nil
}

type decodedResource struct {
	obj *unstructured.Unstructured
	gvk *schema.GroupVersionKind
}

func decodeYAML(data []byte) ([]decodedResource, error) {
	decoder := yaml.NewYAMLToJSONDecoder(bytes.NewReader(data))
	var objs []decodedResource
	for {
		obj := &unstructured.Unstructured{}
		err := decoder.Decode(obj)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if obj.GetAPIVersion() == "" && obj.GetKind() == "" {
			continue
		}
		if obj.GetAPIVersion() == "" || obj.GetKind() == "" {
			return nil, fmt.Errorf("resource must have apiVersion and kind")
		}
		gvk := &schema.GroupVersionKind{Kind: obj.GetKind()}
		parts := strings.SplitN(obj.GetAPIVersion(), "/", 2)
		if len(parts) == 1 {
			gvk.Version = parts[0]
		} else {
			gvk.Group, gvk.Version = parts[0], parts[1]
		}
		objs = append(objs, decodedResource{obj: obj, gvk: gvk})
	}
	return objs, nil
}

func kindToResource(kind string) string {
	k := strings.ToLower(kind)
	switch k {
	case "namespace", "ingress", "configmap", "endpoints":
		return k + "es"
	case "service", "deployment", "daemonset", "statefulset", "replicaset", "secret", "pod":
		return k + "s"
	default:
		if strings.HasSuffix(k, "s") {
			return k + "es"
		}
		return k + "s"
	}
}

func buildManifestWork(objs []decodedResource, workName string) (*workv1.ManifestWork, error) {
	manifests := make([]workv1.Manifest, 0, len(objs))
	manifestConfigs := make([]workv1.ManifestConfigOption, 0, len(objs))

	for _, r := range objs {
		manifests = append(manifests, workv1.Manifest{RawExtension: runtime.RawExtension{Object: r.obj}})
		resource := kindToResource(r.gvk.Kind)
		ri := workv1.ResourceIdentifier{
			Group:     r.gvk.Group,
			Resource:  resource,
			Name:      r.obj.GetName(),
			Namespace: r.obj.GetNamespace(),
		}
		manifestConfigs = append(manifestConfigs, workv1.ManifestConfigOption{
			ResourceIdentifier: ri,
			FeedbackRules: []workv1.FeedbackRule{
				{Type: workv1.JSONPathsType, JsonPaths: []workv1.JsonPath{{Name: "status", Path: ".status"}}},
			},
			UpdateStrategy: &workv1.UpdateStrategy{Type: workv1.UpdateStrategyTypeServerSideApply},
		})
	}

	return &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{Name: workName},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{Manifests: manifests},
			DeleteOption: &workv1.DeleteOption{
				PropagationPolicy: workv1.DeletePropagationPolicyTypeForeground,
			},
			ManifestConfigs: manifestConfigs,
		},
	}, nil
}

func newWorkClient(ctx context.Context, opts Options) (workv1client.WorkV1Interface, error) {
	timeout := opts.ServerHealthTimeout
	if timeout == 0 {
		timeout = 20 * time.Second
	}
	sourceID := opts.SourceID
	if sourceID == "" {
		sourceID = "mw-client-example"
	}

	apiClient := openapi.NewAPIClient(&openapi.Configuration{
		DefaultHeader: make(map[string]string),
		UserAgent:     "maestro-apply/1.0",
		Debug:         false,
		Servers:       openapi.ServerConfigurations{{URL: opts.MaestroServer, Description: "maestro"}},
		OperationServers: map[string]openapi.ServerConfigurations{},
		HTTPClient: &http.Client{
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: opts.InsecureSkipVerify}},
			Timeout:   10 * time.Second,
		},
	})

	logger, err := logging.NewStdLoggerBuilder().Build()
	if err != nil {
		return nil, err
	}

	grpcOptions := &grpc.GRPCOptions{
		Dialer:                   &grpc.GRPCDialer{},
		ServerHealthinessTimeout: &timeout,
	}
	grpcOptions.Dialer.URL = opts.GRPCServer

	return grpcsource.NewMaestroGRPCSourceWorkClient(ctx, logger, apiClient, grpcOptions, sourceID)
}
