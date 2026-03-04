package patch

import (
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "patch",
		Short: "Patch resources",
	}

	cmd.AddCommand(newPatchClusterCommand())
	return cmd
}
