package discover

import (
	"fmt"
	"time"

	maestroapply "github.com/openshift-online/maestro/pkg/client/apply"

	"github.com/spf13/cobra"
)

type Options struct {
	MaestroServer      string
	MaestroGRPCServer  string
	MaestroConsumer    string
	MaestroInsecureTLS bool
	Namespace          string
}

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Discover existing resources and register them with Maestro",
	}

	cmd.AddCommand(newDiscoverClusterCommand())
	return cmd
}

func newDiscoverClusterCommand() *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:   "cluster <name>",
		Short: "Discover an existing HostedCluster and register it with Maestro (read-only)",
		Long: `Discover registers an existing HostedCluster on a consumer cluster with Maestro
using a ReadOnly ManifestWork. The Maestro agent observes the live resource and
reports its status back without modifying it.

This avoids SSA field ownership conflicts that occur when applying a full
HostedCluster spec via ServerSideApply.`,
		Args: cobra.ExactArgs(1),
	}

	cmd.Flags().StringVar(&opts.MaestroServer, "maestro-server", opts.MaestroServer, "Maestro HTTP API URL")
	cmd.Flags().StringVar(&opts.MaestroGRPCServer, "maestro-grpc-server", opts.MaestroGRPCServer, "Maestro gRPC server address (default: 127.0.0.1:8090)")
	cmd.Flags().StringVar(&opts.MaestroConsumer, "maestro-consumer", opts.MaestroConsumer, "Maestro consumer (cluster) name")
	cmd.Flags().BoolVar(&opts.MaestroInsecureTLS, "maestro-insecure-skip-verify", opts.MaestroInsecureTLS, "Skip TLS verification for Maestro HTTP API")
	cmd.Flags().StringVarP(&opts.Namespace, "namespace", "n", "clusters", "Namespace of the HostedCluster on the consumer cluster")

	_ = cmd.MarkFlagRequired("maestro-server")
	_ = cmd.MarkFlagRequired("maestro-consumer")

	cmd.RunE = func(c *cobra.Command, args []string) error {
		clusterName := args[0]
		return runDiscoverCluster(c, opts, clusterName)
	}

	return cmd
}

func runDiscoverCluster(c *cobra.Command, opts *Options, clusterName string) error {
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

	if err := maestroapply.ImportCluster(c.Context(), applyOpts, clusterName, opts.Namespace); err != nil {
		return fmt.Errorf("discover cluster: %w", err)
	}

	fmt.Printf("Discovered HostedCluster %s/%s from consumer %s (read-only)\n", opts.Namespace, clusterName, opts.MaestroConsumer)
	return nil
}
