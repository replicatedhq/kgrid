package cli

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func GetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "get",
		Short:         "get a list of resources",
		SilenceErrors: true,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
			os.Exit(1)
		},
	}

	cmd.PersistentFlags().StringP("grid", "g", "", "Name of the grid")
	cmd.PersistentFlags().StringP("cluster", "c", "", "Name of the cluster")
	cmd.PersistentFlags().StringP("output", "o", "", "Output format (empty or json)")

	cmd.AddCommand(GetGridsCmd())
	cmd.AddCommand(GetNamespacesCmd())

	return cmd
}
