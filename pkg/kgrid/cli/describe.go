package cli

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func DescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "describe",
		Short:         "describe a resource",
		SilenceErrors: true,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
			os.Exit(1)
		},
	}

	cmd.PersistentFlags().StringP("output", "o", "", "Output format (empty or json)")

	cmd.AddCommand(DescribeGridCmd())

	return cmd
}
