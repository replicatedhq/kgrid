package cli

import (
	"encoding/json"
	"fmt"

	"github.com/replicatedhq/kgrid/pkg/kgrid/grid"
	"github.com/replicatedhq/kgrid/pkg/kgrid/grid/types"
	"github.com/replicatedhq/kgrid/pkg/kgrid/print"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func GetGridsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "grids",
		Aliases: []string{
			"grid",
		},
		Short:         "List the known grids",
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

			if v.GetString("output") == "json" {
				printGridsJSON(grids)
			} else {
				printGridsTable(grids)
			}

			return nil
		},
	}

	return cmd
}

func printGridsJSON(grids []*types.GridConfig) {
	str, _ := json.MarshalIndent(grids, "", "    ")
	fmt.Println(string(str))
}

func printGridsTable(grids []*types.GridConfig) {
	if len(grids) == 0 {
		fmt.Println("No grids found")
		return
	}

	w := print.NewTabWriter()
	defer w.Flush()

	fmtColumns := "%s\n"
	fmt.Fprintf(w, fmtColumns, "NAME")
	for _, g := range grids {
		fmt.Fprintf(w, fmtColumns, g.Name)
	}
}
