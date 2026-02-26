package azure

import (
	hypershiftazure "github.com/openshift/hypershift/cmd/cluster/azure"
	"github.com/openshift/hypershift/cmd/cluster/core"
	"github.com/openshift/hypershift/cmd/log"
	"github.com/openshift/hypershift/cmd/util"
	"github.com/openshift/hypershift/product-cli/pkg/maestro"

	"github.com/spf13/cobra"
)

func NewDestroyCommand(opts *core.DestroyOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "azure",
		Short:        "Destroys a HostedCluster and its associated infrastructure on Azure",
		SilenceUsage: true,
	}

	opts.AzurePlatform.Location = "eastus"
	cmd.Flags().StringVar(&opts.AzurePlatform.CredentialsFile, "azure-creds", opts.AzurePlatform.CredentialsFile, util.AzureCredsDestroyDescription)
	cmd.Flags().StringVar(&opts.AzurePlatform.Location, "location", opts.AzurePlatform.Location, util.LocationDestroyDescription)
	cmd.Flags().StringVar(&opts.AzurePlatform.ResourceGroupName, "resource-group-name", opts.AzurePlatform.ResourceGroupName, util.ResourceGroupNameDestroyDescription)
	cmd.Flags().BoolVar(&opts.AzurePlatform.PreserveResourceGroup, "preserve-resource-group", opts.AzurePlatform.PreserveResourceGroup, util.PreserveResourceGroupDescription)

	_ = cmd.MarkFlagRequired("azure-creds")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if done, err := maestro.DestroyViaMaestro(ctx, opts); done {
			if err != nil {
				log.Log.Error(err, "Failed to delete ManifestWork from Maestro")
				return err
			}
			return nil
		}
		return hypershiftazure.DestroyCluster(ctx, opts)
	}

	return cmd
}
