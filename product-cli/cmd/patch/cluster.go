package patch

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	maestroapply "github.com/openshift-online/maestro/pkg/client/apply"

	"github.com/spf13/cobra"
)

// Options holds flags for patch cluster.
type Options struct {
	MaestroServer      string
	MaestroGRPCServer  string
	MaestroConsumer    string
	MaestroInsecureTLS bool
	Name               string
	Patch              string
	PatchFile          string
}

func newPatchClusterCommand() *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:   "cluster <name>",
		Short: "Patch HostedCluster spec in Maestro ManifestWork",
		Long: `Apply a JSON merge patch to the HostedCluster spec in the ManifestWork.

The patch is RFC 7396 merge patch format. Only the HostedCluster resource in the
ManifestWork is patched; other resources (Secrets, etc.) are unchanged.

Examples:
  # Patch node pool replicas
  hcp patch cluster mycluster --patch '{"spec":{"nodePools":[{"name":"nodepool-1","replicas":3}]}}' \
    --maestro-server https://... --maestro-consumer mce-1

  # Patch from file
  hcp patch cluster mycluster --patch-file patch.json --maestro-server https://... --maestro-consumer mce-1`,
		Args: cobra.ExactArgs(1),
	}

	cmd.Flags().StringVar(&opts.MaestroServer, "maestro-server", opts.MaestroServer, "Maestro HTTP API URL")
	cmd.Flags().StringVar(&opts.MaestroGRPCServer, "maestro-grpc-server", opts.MaestroGRPCServer, "Maestro gRPC server address (default: 127.0.0.1:8090)")
	cmd.Flags().StringVar(&opts.MaestroConsumer, "maestro-consumer", opts.MaestroConsumer, "Maestro consumer (cluster) name")
	cmd.Flags().BoolVar(&opts.MaestroInsecureTLS, "maestro-insecure-skip-verify", opts.MaestroInsecureTLS, "Skip TLS verification for Maestro HTTP API")
	cmd.Flags().StringVar(&opts.Patch, "patch", opts.Patch, "JSON merge patch to apply to HostedCluster spec")
	cmd.Flags().StringVar(&opts.PatchFile, "patch-file", opts.PatchFile, "Path to file containing JSON merge patch")

	_ = cmd.MarkFlagRequired("maestro-server")
	_ = cmd.MarkFlagRequired("maestro-consumer")
	cmd.MarkFlagsOneRequired("patch", "patch-file")
	cmd.MarkFlagsMutuallyExclusive("patch", "patch-file")

	cmd.RunE = func(c *cobra.Command, args []string) error {
		opts.Name = args[0]
		return runPatchCluster(c.Context(), opts)
	}

	return cmd
}

func runPatchCluster(ctx context.Context, opts *Options) error {
	var patchJSON []byte
	var err error
	if opts.PatchFile != "" {
		patchJSON, err = os.ReadFile(opts.PatchFile)
		if err != nil {
			return fmt.Errorf("read patch file: %w", err)
		}
	} else {
		patchJSON = []byte(opts.Patch)
	}

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

	if err := maestroapply.PatchHostedClusterInWork(ctx, applyOpts, patchJSON); err != nil {
		return fmt.Errorf("patch cluster: %w", err)
	}

	fmt.Printf("Patched HostedCluster in ManifestWork %s (consumer: %s)\n", workName, opts.MaestroConsumer)
	return nil
}
