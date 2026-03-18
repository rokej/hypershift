package list

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	maestroapply "github.com/openshift-online/maestro/pkg/client/apply"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/spf13/cobra"
	workv1 "open-cluster-management.io/api/work/v1"
)

// Options holds flags for list commands.
type Options struct {
	MaestroServer      string
	MaestroGRPCServer  string
	MaestroConsumer    string
	MaestroInsecureTLS bool
}

func NewCommand() *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List resources",
	}

	cmd.AddCommand(newListClustersCommand(opts))
	return cmd
}

func newListClustersCommand(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clusters",
		Short: "List hosted clusters from Maestro (single consumer or all consumers)",
	}

	cmd.Flags().StringVar(&opts.MaestroServer, "maestro-server", opts.MaestroServer, "Maestro HTTP API URL")
	cmd.Flags().StringVar(&opts.MaestroGRPCServer, "maestro-grpc-server", opts.MaestroGRPCServer, "Maestro gRPC server address (default: 127.0.0.1:8090)")
	cmd.Flags().StringVar(&opts.MaestroConsumer, "maestro-consumer", opts.MaestroConsumer, "Maestro consumer (cluster) name. If not set, lists clusters from all consumers")
	cmd.Flags().BoolVar(&opts.MaestroInsecureTLS, "maestro-insecure-skip-verify", opts.MaestroInsecureTLS, "Skip TLS verification for Maestro HTTP API")

	_ = cmd.MarkFlagRequired("maestro-server")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		grpcServer := opts.MaestroGRPCServer
		if grpcServer == "" {
			grpcServer = "127.0.0.1:8090"
		}

		applyOpts := maestroapply.Options{
			MaestroServer:       opts.MaestroServer,
			GRPCServer:          grpcServer,
			ConsumerName:        opts.MaestroConsumer,
			InsecureSkipVerify:  opts.MaestroInsecureTLS,
			ServerHealthTimeout: 20 * time.Second,
		}

		works, err := maestroapply.ListManifestWorks(ctx, applyOpts)
		if err != nil {
			return fmt.Errorf("list clusters: %w", err)
		}

		if len(works) == 0 {
			fmt.Println("No hosted clusters found")
			return nil
		}

		showConsumer := opts.MaestroConsumer == "" || hasMultipleConsumers(works)
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 3, ' ', 0)

		if showConsumer {
			fmt.Fprintln(w, "NAMESPACE\tNAME\tCONSUMER\tVERSION\tKUBECONFIG\tPROGRESS\tAVAILABLE\tPROGRESSING\tMESSAGE")
		} else {
			fmt.Fprintln(w, "NAMESPACE\tNAME\tVERSION\tKUBECONFIG\tPROGRESS\tAVAILABLE\tPROGRESSING\tMESSAGE")
		}

		for _, wi := range works {
			hc := extractHostedClusterFromWork(&wi.Work)
			var namespace string
			if hc != nil {
				namespace = hc.GetNamespace()
			}
			version := getVersion(hc)
			kubeconfig := getKubeconfig(hc)
			progress := getProgress(hc)
			available := getConditionStatus(hc, "Available")
			progressing := getConditionStatus(hc, "Progressing")
			message := getConditionMessage(hc, "Available")

			if showConsumer {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					namespace, wi.ClusterName, wi.Consumer, version, kubeconfig, progress, available, progressing, message)
			} else {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					namespace, wi.ClusterName, version, kubeconfig, progress, available, progressing, message)
			}
		}
		return w.Flush()
	}

	return cmd
}

func hasMultipleConsumers(works []maestroapply.WorkInfo) bool {
	if len(works) <= 1 {
		return false
	}
	first := works[0].Consumer
	for _, w := range works[1:] {
		if w.Consumer != first {
			return true
		}
	}
	return false
}

func extractHostedClusterFromWork(mw *workv1.ManifestWork) *unstructured.Unstructured {
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
		if gvk.Kind == "HostedCluster" && gvk.Group == "hypershift.openshift.io" {
			hcObj = obj
			hcOrdinal = int32(i)
			break
		}
	}
	if hcObj == nil {
		return nil
	}

	for _, mc := range mw.Status.ResourceStatus.Manifests {
		rm := mc.ResourceMeta
		if rm.Ordinal == hcOrdinal || (rm.Kind == "HostedCluster" && rm.Group == "hypershift.openshift.io") {
			for _, v := range mc.StatusFeedbacks.Values {
				if v.Name == "status" && v.Value.JsonRaw != nil {
					if hcObj.Object == nil {
						hcObj.Object = make(map[string]interface{})
					}
					var statusObj interface{}
					if err := json.Unmarshal([]byte(*v.Value.JsonRaw), &statusObj); err == nil {
						hcObj.Object["status"] = statusObj
					}
					break
				}
			}
			break
		}
	}
	return hcObj
}

func getVersion(hc *unstructured.Unstructured) string {
	if hc == nil {
		return ""
	}
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
	if hc == nil {
		return ""
	}
	name, found, err := unstructured.NestedString(hc.Object, "status", "kubeconfig", "name")
	if err != nil || !found {
		return ""
	}
	return name
}

func getProgress(hc *unstructured.Unstructured) string {
	if hc == nil {
		return ""
	}
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
	if hc == nil {
		return ""
	}
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
	if hc == nil {
		return ""
	}
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
