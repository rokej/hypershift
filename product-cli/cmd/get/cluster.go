package get

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	maestroapply "github.com/openshift-online/maestro/pkg/client/apply"
	workv1 "open-cluster-management.io/api/work/v1"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

// Options holds flags for get cluster.
type Options struct {
	MaestroServer      string
	MaestroGRPCServer  string
	MaestroConsumer    string
	MaestroInsecureTLS bool
	Name               string
}

func newGetClusterCommand() *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:   "cluster <name>",
		Short: "Get HostedCluster resource and status from Maestro",
		Args:  cobra.ExactArgs(1),
	}

	cmd.Flags().StringVar(&opts.MaestroServer, "maestro-server", opts.MaestroServer, "Maestro HTTP API URL")
	cmd.Flags().StringVar(&opts.MaestroGRPCServer, "maestro-grpc-server", opts.MaestroGRPCServer, "Maestro gRPC server address (default: 127.0.0.1:8090)")
	cmd.Flags().StringVar(&opts.MaestroConsumer, "maestro-consumer", opts.MaestroConsumer, "Maestro consumer (cluster) name")
	cmd.Flags().BoolVar(&opts.MaestroInsecureTLS, "maestro-insecure-skip-verify", opts.MaestroInsecureTLS, "Skip TLS verification for Maestro HTTP API")

	_ = cmd.MarkFlagRequired("maestro-server")
	_ = cmd.MarkFlagRequired("maestro-consumer")

	cmd.RunE = func(c *cobra.Command, args []string) error {
		opts.Name = args[0]
		return runGetCluster(c.Context(), opts)
	}

	return cmd
}

func runGetCluster(ctx context.Context, opts *Options) error {
	grpcServer := opts.MaestroGRPCServer
	if grpcServer == "" {
		grpcServer = "127.0.0.1:8090"
	}

	workName := opts.Name
	if !strings.HasSuffix(workName, "-work") {
		workName = workName + "-work"
	}

	applyOpts := maestroapply.Options{
		MaestroServer:       opts.MaestroServer,
		GRPCServer:          grpcServer,
		ConsumerName:        opts.MaestroConsumer,
		WorkName:            workName,
		InsecureSkipVerify:  opts.MaestroInsecureTLS,
		ServerHealthTimeout: 20 * time.Second,
	}

	mw, err := maestroapply.GetManifestWork(ctx, applyOpts)
	if err != nil {
		return fmt.Errorf("get cluster: %w", err)
	}

	hc, statusYAML, err := extractHostedCluster(mw)
	if err != nil {
		return fmt.Errorf("extract HostedCluster: %w", err)
	}

	// Output: HostedCluster with status
	fmt.Println("---")
	fmt.Println("# HostedCluster spec and status from Maestro ManifestWork")
	fmt.Println("---")
	if statusYAML != "" {
		// Merge status into the object
		if hc.Object == nil {
			hc.Object = make(map[string]interface{})
		}
		var statusObj interface{}
		if err := json.Unmarshal([]byte(statusYAML), &statusObj); err == nil {
			hc.Object["status"] = statusObj
		}
	}

	out, err := runtime.Encode(unstructured.UnstructuredJSONScheme, hc)
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	var outMap map[string]interface{}
	if err := json.Unmarshal(out, &outMap); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	yamlOut, err := yaml.Marshal(outMap)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}
	fmt.Print(string(yamlOut))
	return nil
}

func extractHostedCluster(mw *workv1.ManifestWork) (*unstructured.Unstructured, string, error) {
	hostedClusterGVK := "hypershift.openshift.io/v1beta1"
	hostedClusterKind := "HostedCluster"

	var hcObj *unstructured.Unstructured
	var hcOrdinal int32 = -1

	for i, m := range mw.Spec.Workload.Manifests {
		var obj *unstructured.Unstructured
		if m.RawExtension.Object != nil {
			if u, ok := m.RawExtension.Object.(*unstructured.Unstructured); ok {
				obj = u
			}
		}
		if obj == nil && m.RawExtension.Raw != nil {
			obj = &unstructured.Unstructured{}
			if err := json.Unmarshal(m.RawExtension.Raw, obj); err != nil {
				continue
			}
		}
		if obj == nil {
			continue
		}
		gvk := obj.GroupVersionKind()
		if gvk.Kind == hostedClusterKind && (gvk.Group == "hypershift.openshift.io" || gvk.GroupVersion().String() == hostedClusterGVK) {
			hcObj = obj
			hcOrdinal = int32(i)
			break
		}
	}

	if hcObj == nil {
		return nil, "", fmt.Errorf("no HostedCluster found in ManifestWork")
	}

	// Find status from ResourceStatus
	var statusJSON string
	for _, mc := range mw.Status.ResourceStatus.Manifests {
		rm := mc.ResourceMeta
		if rm.Ordinal == hcOrdinal || (rm.Kind == hostedClusterKind && rm.Group == "hypershift.openshift.io") {
			for _, v := range mc.StatusFeedbacks.Values {
				if v.Name == "status" && v.Value.JsonRaw != nil {
					statusJSON = *v.Value.JsonRaw
					break
				}
			}
			break
		}
	}

	return hcObj, statusJSON, nil
}
