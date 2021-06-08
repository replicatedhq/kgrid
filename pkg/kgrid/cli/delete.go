package cli

import (
	"io/ioutil"

	"github.com/replicatedhq/kgrid/pkg/kgrid/grid"
	"github.com/replicatedhq/kgrid/pkg/kgrid/grid/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"sigs.k8s.io/yaml"
)

func DeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "delete",
		Short:         "Delete a grid",
		SilenceErrors: true,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			gridSpecData, err := ioutil.ReadFile(v.GetString("from-yaml"))
			if err != nil {
				return err
			}

			gridSpec := &types.Grid{}
			if err := yaml.Unmarshal(gridSpecData, gridSpec); err != nil {
				return err
			}

			if err := grid.Delete(v.GetString("config-file"), gridSpec); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringP("name", "n", "", "Name of the grid, overriding the name in the yaml metadata.name field")
	cmd.Flags().String("from-yaml", "", "Path to YAML manifest describing the grid to delete")
	cmd.Flags().String("like", "", "Name of an existing grid to clone, into a new grid")

	return cmd
}
