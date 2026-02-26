package get

import (
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get resources",
	}

	cmd.AddCommand(newGetClusterCommand())
	return cmd
}
