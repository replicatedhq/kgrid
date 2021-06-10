package cli

import (
	"fmt"

	"github.com/replicatedhq/kgrid/pkg/buildversion"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func VersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "version",
		Short:         "Show kgrid version",
		SilenceErrors: true,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(buildversion.Version())
			return nil
		},
	}

	return cmd
}
