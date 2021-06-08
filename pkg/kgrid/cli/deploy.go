package cli

import (
	"io/ioutil"

	"github.com/pkg/errors"
	"github.com/replicatedhq/kgrid/pkg/kgrid/app"
	"github.com/replicatedhq/kgrid/pkg/kgrid/grid"
	"github.com/replicatedhq/kgrid/pkg/kgrid/grid/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"sigs.k8s.io/yaml"
)

func DeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "deploy",
		Short:         "Deploy an application to a grid",
		SilenceErrors: true,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			return deployApp(v.GetString("config-file"), v.GetString("grid"), v.GetString("app"))
		},
	}

	cmd.Flags().StringP("grid", "g", "", "Name of the grid")
	cmd.Flags().String("app", "", "Path to YAML manifest describing the application to deploy")

	return cmd
}

func deployApp(configFile string, gridName string, appSpecFilename string) error {
	data, err := ioutil.ReadFile(appSpecFilename)
	if err != nil {
		return errors.Wrap(err, "failed to read app spec file")
	}

	application := types.Application{}
	if err := yaml.Unmarshal(data, &application); err != nil {
		return errors.Wrap(err, "failed to unmarshal app spec")
	}

	grids, err := grid.List(configFile)
	if err != nil {
		return errors.Wrap(err, "failed to list grids")
	}

	for _, g := range grids {
		if g.Name == gridName {
			if err := app.Deploy(g, &application); err != nil {
				return errors.Wrap(err, "failed to deploy app")
			}

			return nil
		}
	}

	return errors.New("unable to find grid")
}
