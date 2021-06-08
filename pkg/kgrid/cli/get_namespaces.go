package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/replicatedhq/kgrid/pkg/kgrid/cluster"
	"github.com/replicatedhq/kgrid/pkg/kgrid/grid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/printers"
)

func GetNamespacesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "namespaces",
		Aliases: []string{
			"namespace",
			"ns",
		},
		Short:         "List the namespaces in a cluster on the grid",
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
				if g.Name == v.GetString("grid") {
					for _, c := range g.ClusterConfigs {
						if c.Name == v.GetString("cluster") {
							namespaces, err := cluster.ListNamespaces(c)
							if err != nil {
								return err
							}

							if v.GetString("output") == "json" {
								printNamespacesJSON(namespaces)
							} else {
								printNamespacesTable(namespaces)
							}

							return nil
						}
					}

					return errors.New("cluster not found")
				}
			}

			return errors.New("grid not found")
		},
	}

	cmd.Flags().StringP("cluster", "c", "", "The name of the cluster to get namespaces in")

	cmd.MarkFlagRequired("cluster")

	return cmd
}

func printNamespacesTable(namespaces *corev1.NamespaceList) {
	if len(namespaces.Items) == 0 {
		fmt.Println("No namespaces found")
		return
	}

	p := printers.NewTablePrinter(printers.PrintOptions{})
	for _, ns := range namespaces.Items {
		p.PrintObj(&ns, os.Stdout)
	}
}

func printNamespacesJSON(namespaces *corev1.NamespaceList) {

}
