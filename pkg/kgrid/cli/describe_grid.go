package cli

import (
	"fmt"
	"strings"

	"github.com/replicatedhq/kgrid/pkg/kgrid/grid"
	"github.com/replicatedhq/kgrid/pkg/kgrid/grid/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func DescribeGridCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "grid",
		Short:         "Describe a grid",
		SilenceErrors: true,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			grids, err := grid.List(v.GetString("config-file"))
			if err != nil {
				return err
			}

			for _, g := range grids {
				if g.Name == args[0] {
					if v.GetString("output") == "json" {
						printJSONGridDescription(g)
					} else {
						printTextGridDescription(g)
					}
				}
			}

			return nil
		},
	}

	return cmd
}

func printTextGridDescription(g *types.GridConfig) {
	clusters := []string{}
	for _, c := range g.ClusterConfigs {
		renderedCluster := fmt.Sprintf("  - Name: %s\n    Provider: %s\n",
			c.Name, c.Provider)

		clusters = append(clusters, renderedCluster)
	}

	fmt.Printf(`Grid Name: %s
Clusters:
%s	
`,
		g.Name, strings.Join(clusters, "\n"))
}

func printJSONGridDescription(g *types.GridConfig) {
	fmt.Printf("not implemented\n")
}
