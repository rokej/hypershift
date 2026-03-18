package aws

import (
	"context"
	"fmt"
	"time"

	hypershiftaws "github.com/openshift/hypershift/cmd/cluster/aws"
	"github.com/openshift/hypershift/cmd/cluster/core"
	"github.com/openshift/hypershift/support/config"

	maestroapply "github.com/openshift-online/maestro/pkg/client/apply"

	"github.com/spf13/cobra"
)

func NewCreateCommand(opts *core.RawCreateOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "aws",
		Short:        "Creates basic functional HostedCluster resources on AWS",
		SilenceUsage: true,
	}

	opts.ReleaseStream = config.DefaultReleaseStream

	awsOpts := hypershiftaws.DefaultOptions()

	hypershiftaws.BindOptions(awsOpts, cmd.Flags())
	cmd.Flags().StringVar(&opts.TargetCluster, "target-cluster", opts.TargetCluster, "When set, renders the cluster manifests (including secrets) to a YAML file instead of applying. Output is saved to <target-cluster>.yaml")
	cmd.Flags().StringVar(&opts.MaestroServer, "maestro-server", opts.MaestroServer, "Maestro HTTP API URL. When set with --target-cluster, applies the rendered manifests to Maestro")
	cmd.Flags().StringVar(&opts.MaestroGRPCServer, "maestro-grpc-server", opts.MaestroGRPCServer, "Maestro gRPC server address (default: 127.0.0.1:8090)")
	cmd.Flags().StringVar(&opts.MaestroConsumer, "maestro-consumer", opts.MaestroConsumer, "Maestro consumer (cluster) name. Defaults to --target-cluster when not set")
	cmd.Flags().BoolVar(&opts.MaestroInsecureTLS, "maestro-insecure-skip-verify", opts.MaestroInsecureTLS, "Skip TLS verification for Maestro HTTP API")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if opts.Timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
			defer cancel()
		}

		if opts.TargetCluster != "" {
			opts.Render = true
			opts.RenderSensitive = true
			if opts.RenderInto == "" {
				opts.RenderInto = opts.TargetCluster + ".yaml"
			}
		}

		if err := core.CreateCluster(ctx, opts, awsOpts); err != nil {
			opts.Log.Error(err, "Failed to create cluster")
			return err
		}

		if opts.MaestroServer != "" && opts.Render && opts.RenderInto != "" {
			consumer := opts.MaestroConsumer
			if consumer == "" {
				consumer = opts.TargetCluster
			}
			if consumer == "" {
				return fmt.Errorf("--maestro-consumer or --target-cluster is required when using --maestro-server")
			}
			grpcServer := opts.MaestroGRPCServer
			if grpcServer == "" {
				grpcServer = "127.0.0.1:8090"
			}
			applyOpts := maestroapply.Options{
				MaestroServer:       opts.MaestroServer,
				GRPCServer:          grpcServer,
				ConsumerName:        consumer,
				WorkName:            opts.Name + "-work",
				InsecureSkipVerify:  opts.MaestroInsecureTLS,
				ServerHealthTimeout: 20 * time.Second,
			}
			if err := maestroapply.ApplyManifestsFromFile(ctx, opts.RenderInto, applyOpts); err != nil {
				opts.Log.Error(err, "Failed to apply manifests to Maestro")
				return fmt.Errorf("apply to Maestro: %w", err)
			}
			opts.Log.Info("Applied manifests to Maestro", "consumer", consumer, "file", opts.RenderInto)
		}

		return nil
	}

	return cmd
}
