package apply

import (
	"fmt"
	"time"

	maestroapply "github.com/openshift-online/maestro/pkg/client/apply"

	"github.com/spf13/cobra"
)

// Options holds flags for apply commands.
type Options struct {
	MaestroServer      string
	MaestroGRPCServer  string
	MaestroConsumer    string
	MaestroInsecureTLS bool
}

func NewCommand() *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply manifests to Maestro",
	}

	cmd.AddCommand(newApplyManifestsCommand(opts))
	return cmd
}

func newApplyManifestsCommand(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "manifests <file>",
		Short: "Import existing resources into Maestro by applying a YAML file",
		Long: `Apply Kubernetes manifests (e.g. HostedCluster) from a YAML file to Maestro.
This imports existing resources so they appear in "hcp list clusters".

Use this to discover HostedClusters that were created outside of Maestro.
Export the resource from the consumer cluster (e.g. mce-1), then apply:

  1. Export: oc get hostedcluster virt-hcp-1 -n clusters -o yaml > virt-hcp-1.yaml
  2. Edit to remove status, resourceVersion, uid, creationTimestamp (optional; ServerSideApply tolerates them)
  3. Apply: hcp apply manifests virt-hcp-1.yaml --maestro-server https://... --maestro-consumer mce-1

Port-forward Maestro gRPC first: oc port-forward svc/maestro-grpc 8090:8090 -n maestro`,
		Args: cobra.ExactArgs(1),
	}

	cmd.Flags().StringVar(&opts.MaestroServer, "maestro-server", opts.MaestroServer, "Maestro HTTP API URL")
	cmd.Flags().StringVar(&opts.MaestroGRPCServer, "maestro-grpc-server", opts.MaestroGRPCServer, "Maestro gRPC server address (default: 127.0.0.1:8090)")
	cmd.Flags().StringVar(&opts.MaestroConsumer, "maestro-consumer", opts.MaestroConsumer, "Maestro consumer (cluster) name where the resource exists")
	cmd.Flags().BoolVar(&opts.MaestroInsecureTLS, "maestro-insecure-skip-verify", opts.MaestroInsecureTLS, "Skip TLS verification for Maestro HTTP API")

	_ = cmd.MarkFlagRequired("maestro-server")
	_ = cmd.MarkFlagRequired("maestro-consumer")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		filePath := args[0]

		grpcServer := opts.MaestroGRPCServer
		if grpcServer == "" {
			grpcServer = "127.0.0.1:8090"
		}

		applyOpts := maestroapply.Options{
			MaestroServer:        opts.MaestroServer,
			GRPCServer:           grpcServer,
			ConsumerName:         opts.MaestroConsumer,
			InsecureSkipVerify:   opts.MaestroInsecureTLS,
			ServerHealthTimeout:  20 * time.Second,
		}

		if err := maestroapply.ApplyManifestsFromFile(ctx, filePath, applyOpts); err != nil {
			return fmt.Errorf("apply manifests: %w", err)
		}

		fmt.Printf("Applied manifests from %s to Maestro (consumer: %s)\n", filePath, opts.MaestroConsumer)
		return nil
	}

	return cmd
}
