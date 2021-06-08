package app

import (
	"fmt"
	"reflect"

	"github.com/replicatedhq/kgrid/pkg/kgrid/grid/types"
)

func Deploy(g *types.GridConfig, a *types.Application) error {
	completed := map[int]bool{}
	completedChans := make([]chan string, len(g.ClusterConfigs))
	for i := range g.ClusterConfigs {
		completedChans[i] = make(chan string)
		completed[i] = false
	}

	finished := make(chan bool)
	go func() {
		cases := make([]reflect.SelectCase, len(completedChans))
		for i, ch := range completedChans {
			cases[i] = reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(ch),
			}
		}

		for {
			i, completedErr, ok := reflect.Select(cases)
			if ok {
				if completedErr.String() != "" {
					fmt.Printf("cluster %#v failed with error: %s\n", g.ClusterConfigs[i], completedErr.String())
				}

				completed[i] = true
			}

			allCompleted := true
			for _, v := range completed {
				if !v {
					allCompleted = false
				}
			}

			if allCompleted {
				finished <- true
				return
			}
		}
	}()

	// deploy the app
	for i, c := range g.ClusterConfigs {
		if a.Spec.KOTSApplicationSpec != nil {
			go func() {
				err := deployKOTSApplication(c, a.Spec.KOTSApplicationSpec)
				if err != nil {
					completedChans[i] <- err.Error()
				} else {
					completedChans[i] <- ""
				}
			}()
		}
	}

	<-finished

	return nil
}
