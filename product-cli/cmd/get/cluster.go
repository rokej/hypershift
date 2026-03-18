package get

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	maestroapply "github.com/openshift-online/maestro/pkg/client/apply"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/yaml"

	"github.com/spf13/cobra"
	workv1 "open-cluster-management.io/api/work/v1"
)

// Options holds flags for get cluster.
type Options struct {
	MaestroServer      string
	MaestroGRPCServer  string
	MaestroConsumer    string
	MaestroInsecureTLS bool
	Name               string
	Output             string
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
	cmd.Flags().StringVarP(&opts.Output, "output", "o", "", "Output format: table (default) or yaml")

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

	hc, specJSON, statusJSON, err := extractHostedCluster(mw)
	if err != nil {
		return fmt.Errorf("extract HostedCluster: %w", err)
	}

	if hc.Object == nil {
		hc.Object = make(map[string]interface{})
	}
	if specJSON != "" {
		var specObj interface{}
		if err := json.Unmarshal([]byte(specJSON), &specObj); err == nil {
			hc.Object["spec"] = specObj
		}
	}
	if statusJSON != "" {
		var statusObj interface{}
		if err := json.Unmarshal([]byte(statusJSON), &statusObj); err == nil {
			hc.Object["status"] = statusObj
		}
	}

	if opts.Output == "yaml" {
		return printYAML(hc)
	}
	return printTable(hc)
}

func printYAML(hc *unstructured.Unstructured) error {
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
	fmt.Println("---")
	fmt.Println("# HostedCluster spec and status from Maestro ManifestWork")
	fmt.Println("---")
	fmt.Print(string(yamlOut))
	return nil
}

func printTable(hc *unstructured.Unstructured) error {
	namespace := hc.GetNamespace()
	name := hc.GetName()
	version := getVersion(hc)
	kubeconfig := getKubeconfig(hc)
	progress := getProgress(hc)
	available := getConditionStatus(hc, "Available")
	progressing := getConditionStatus(hc, "Progressing")
	message := getConditionMessage(hc, "Available")

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 3, ' ', 0)
	fmt.Fprintln(w, "NAMESPACE\tNAME\tVERSION\tKUBECONFIG\tPROGRESS\tAVAILABLE\tPROGRESSING\tMESSAGE")
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
		namespace, name, version, kubeconfig, progress, available, progressing, message)
	return w.Flush()
}

func getVersion(hc *unstructured.Unstructured) string {
	history, found, err := unstructured.NestedSlice(hc.Object, "status", "version", "history")
	if err != nil || !found {
		return ""
	}
	for _, item := range history {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		state, _, _ := unstructured.NestedString(entry, "state")
		if state == "Completed" {
			ver, _, _ := unstructured.NestedString(entry, "version")
			return ver
		}
	}
	return ""
}

func getKubeconfig(hc *unstructured.Unstructured) string {
	name, found, err := unstructured.NestedString(hc.Object, "status", "kubeconfig", "name")
	if err != nil || !found {
		return ""
	}
	return name
}

func getProgress(hc *unstructured.Unstructured) string {
	history, found, err := unstructured.NestedSlice(hc.Object, "status", "version", "history")
	if err != nil || !found {
		return ""
	}
	for _, item := range history {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		state, _, _ := unstructured.NestedString(entry, "state")
		if state != "" {
			return state
		}
	}
	return ""
}

func getConditionStatus(hc *unstructured.Unstructured, condType string) string {
	conditions, found, err := unstructured.NestedSlice(hc.Object, "status", "conditions")
	if err != nil || !found {
		return ""
	}
	for _, c := range conditions {
		cond, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		t, _, _ := unstructured.NestedString(cond, "type")
		if t == condType {
			s, _, _ := unstructured.NestedString(cond, "status")
			return s
		}
	}
	return ""
}

func getConditionMessage(hc *unstructured.Unstructured, condType string) string {
	conditions, found, err := unstructured.NestedSlice(hc.Object, "status", "conditions")
	if err != nil || !found {
		return ""
	}
	for _, c := range conditions {
		cond, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		t, _, _ := unstructured.NestedString(cond, "type")
		if t == condType {
			m, _, _ := unstructured.NestedString(cond, "message")
			return m
		}
	}
	return ""
}

func extractHostedCluster(mw *workv1.ManifestWork) (*unstructured.Unstructured, string, string, error) {
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
		return nil, "", "", fmt.Errorf("no HostedCluster found in ManifestWork")
	}

	// Find spec and status from ResourceStatus StatusFeedbacks
	var specJSON, statusJSON string
	for _, mc := range mw.Status.ResourceStatus.Manifests {
		rm := mc.ResourceMeta
		if rm.Ordinal == hcOrdinal || (rm.Kind == hostedClusterKind && rm.Group == "hypershift.openshift.io") {
			for _, v := range mc.StatusFeedbacks.Values {
				if v.Value.JsonRaw == nil {
					continue
				}
				switch v.Name {
				case "spec":
					specJSON = *v.Value.JsonRaw
				case "status":
					statusJSON = *v.Value.JsonRaw
				}
			}
			break
		}
	}

	return hcObj, specJSON, statusJSON, nil
}
