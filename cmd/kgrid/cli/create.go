package cli

import (
	"io/ioutil"

	"github.com/pkg/errors"
	"github.com/replicatedhq/kgrid/pkg/kgrid/grid"
	"github.com/replicatedhq/kgrid/pkg/kgrid/grid/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"sigs.k8s.io/yaml"
)

func CreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "create",
		Short:         "Create a new test grid",
		SilenceErrors: true,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			if v.GetString("like") != "" {
				return errors.New("like is not yet supported")
			}

			data, err := ioutil.ReadFile(v.GetString("from-yaml"))
			if err != nil {
				return errors.Wrap(err, "failed to read from-yaml file")
			}

			gridSpec := types.Grid{}
			if err := yaml.Unmarshal(data, &gridSpec); err != nil {
				return errors.Wrapf(err, "failed to unmarshal %s", v.GetString("from-yaml"))
			}

			if v.GetString("name") != "" {
				gridSpec.Name = v.GetString("name")
			}

			if err := grid.Create(v.GetString("config-file"), &gridSpec); err != nil {
				return errors.Wrap(err, "failed to create cluster")
			}

			if v.GetString("app") == "" {
				return nil
			}

			if err := deployApp(v.GetString("config-file"), gridSpec.Name, v.GetString("app")); err != nil {
				return errors.Wrap(err, "failed to deploy app")
			}

			return nil
		},
	}

	cmd.Flags().StringP("name", "n", "", "Name of the grid, overriding the name in the yaml metadata.name field")
	cmd.Flags().String("from-yaml", "", "Path to YAML manifest describing the grid to create")
	cmd.Flags().String("like", "", "Name of an existing grid to clone, into a new grid")
	cmd.Flags().String("app", "", "Path to YAML manifest describing the application to deploy after grid is created")

	return cmd
}
