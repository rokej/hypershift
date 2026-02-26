package agent

import (
	"github.com/openshift/hypershift/cmd/cluster/agent"
	"github.com/openshift/hypershift/cmd/cluster/core"
	"github.com/openshift/hypershift/cmd/log"
	"github.com/openshift/hypershift/product-cli/pkg/maestro"

	"github.com/spf13/cobra"
)

func NewDestroyCommand(opts *core.DestroyOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "agent",
		Short:        "Destroys a HostedCluster and its associated infrastructure on Agent",
		SilenceUsage: true,
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if done, err := maestro.DestroyViaMaestro(ctx, opts); done {
			if err != nil {
				log.Log.Error(err, "Failed to delete ManifestWork from Maestro")
				return err
			}
			return nil
		}

		if err := agent.DestroyCluster(ctx, opts); err != nil {
			log.Log.Error(err, "Failed to destroy cluster")
			return err
		}

		return nil
	}

	return cmd
}
