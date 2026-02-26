package list

import (
	"fmt"
	"time"

	maestroapply "github.com/openshift-online/maestro/pkg/client/apply"

	"github.com/spf13/cobra"
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
			MaestroServer:        opts.MaestroServer,
			GRPCServer:           grpcServer,
			ConsumerName:         opts.MaestroConsumer,
			InsecureSkipVerify:   opts.MaestroInsecureTLS,
			ServerHealthTimeout:  20 * time.Second,
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
		if showConsumer {
			fmt.Printf("%-30s %-30s %-40s %s\n", "NAME", "CONSUMER", "WORK", "CREATED")
			fmt.Printf("%-30s %-30s %-40s %s\n", "----", "--------", "----", "------")
			for _, w := range works {
				fmt.Printf("%-30s %-30s %-40s %s\n", w.ClusterName, w.Consumer, w.Name, w.Created)
			}
		} else {
			fmt.Printf("%-40s %-40s %s\n", "NAME", "WORK", "CREATED")
			fmt.Printf("%-40s %-40s %s\n", "----", "----", "------")
			for _, w := range works {
				fmt.Printf("%-40s %-40s %s\n", w.ClusterName, w.Name, w.Created)
			}
		}
		return nil
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
