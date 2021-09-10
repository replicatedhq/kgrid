package cli

import (
	"io/ioutil"

	"github.com/pkg/errors"
	"github.com/replicatedhq/kgrid/pkg/kgrid/grid"
	"github.com/replicatedhq/kgrid/pkg/kgrid/grid/types"
	"github.com/replicatedhq/kgrid/pkg/kgrid/logger"
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
		RunE: func(cmd *cobra.Command, args []string) (testError error) {
			v := viper.GetViper()

			if v.GetString("like") != "" {
				testError = errors.New("like is not yet supported")
				return
			}

			data, err := ioutil.ReadFile(v.GetString("from-yaml"))
			if err != nil {
				testError = errors.Wrap(err, "failed to read from-yaml file")
				return
			}

			gridSpec := types.Grid{}
			if err := yaml.Unmarshal(data, &gridSpec); err != nil {
				testError = errors.Wrapf(err, "failed to unmarshal %s", v.GetString("from-yaml"))
				return
			}

			if len(gridSpec.Spec.Clusters) == 0 {
				testError = errors.New("no clusters defined in spec")
				return
			}

			if v.GetString("name") != "" {
				gridSpec.Name = v.GetString("name")
			}

			var application *types.Application
			if v.GetString("app") != "" {
				data, err := ioutil.ReadFile(v.GetString("app"))
				if err != nil {
					testError = errors.Wrap(err, "failed to read app spec file")
					return
				}

				application = &types.Application{}
				if err := yaml.Unmarshal(data, &application); err != nil {
					testError = errors.Wrap(err, "failed to unmarshal app spec")
					return
				}
			}

			log := logger.NewLogger(gridSpec.Spec.Clusters[0].Logger)
			if application == nil {
				log.StartThread("Creating clusters for %s", gridSpec.Name)
				defer func() {
					resultMark := ":white_check_mark:"
					if testError != nil {
						resultMark = ":x:"
					}
					log.FinishThread("%s Creating clusters for %s", resultMark, gridSpec.Name)
				}()
			} else {
				log.StartThread("Testing app %s", getAppDisplayName(*application))
				defer func() {
					resultMark := ":white_check_mark:"
					if testError != nil {
						resultMark = ":x:"
					}
					log.FinishThread("%s Testing app %s", resultMark, getAppDisplayName(*application))
				}()
			}

			if err := grid.Create(v.GetString("config-file"), &gridSpec, log); err != nil {
				testError = errors.Wrap(err, "failed to create cluster")
				return
			}

			if v.GetString("app") == "" {
				return
			}

			if err := deployApp(v.GetString("config-file"), gridSpec.Name, v.GetString("app"), log); err != nil {
				testError = errors.Wrap(err, "failed to deploy app")
				return
			}

			return
		},
	}

	cmd.Flags().StringP("name", "n", "", "Name of the grid, overriding the name in the yaml metadata.name field")
	cmd.Flags().String("from-yaml", "", "Path to YAML manifest describing the grid to create")
	cmd.Flags().String("like", "", "Name of an existing grid to clone, into a new grid")
	cmd.Flags().String("app", "", "Path to YAML manifest describing the application to deploy after grid is created")

	return cmd
}

func getAppDisplayName(application types.Application) string {
	if application.Spec.KOTSApplicationSpec != nil {
		return application.Spec.KOTSApplicationSpec.App
	}
	return ""
}
