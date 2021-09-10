package app

import (
	"encoding/json"
	"reflect"
	"time"

	"github.com/pkg/errors"
	"github.com/replicatedhq/kgrid/pkg/kgrid/grid/types"
	"github.com/replicatedhq/kgrid/pkg/kgrid/logger"
)

func Deploy(g *types.GridConfig, a *types.Application, log logger.Logger) (finalError error) {
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
					finalError = errors.New("application failed to deploy")
					log.Info("deploy to cluster %s failed with error: %s\n", g.ClusterConfigs[i].Name, completedErr.String())
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
				pathToKOTSBinary, err := downloadKOTSBinary(a.Spec.KOTSApplicationSpec.Version)
				if err != nil {
					completedChans[i] <- errors.Wrapf(err, "failed to get kots %s binary", a.Spec.KOTSApplicationSpec.Version).Error()
					return
				}

				err = deployKOTSApplication(c, a.Spec.KOTSApplicationSpec, pathToKOTSBinary, log)
				if err != nil {
					completedChans[i] <- err.Error()
					return
				}

				waitUntil := time.Now().Add(5 * time.Minute)
				for {
					appStatus, err := getKOTSApplicationStatus(c, a.Spec.KOTSApplicationSpec, pathToKOTSBinary, log)
					if err != nil {
						completedChans[i] <- err.Error()
						return
					}

					statusString, _ := json.MarshalIndent(appStatus, "", "  ")
					log.Info("```%s```", statusString)
					if appStatus.AppStatus.State == "ready" {
						completedChans[i] <- ""
						return
					}

					if time.Now().After(waitUntil) {
						completedChans[i] <- "timed out waiting for app ready status"
						return
					}

					time.Sleep(10 * time.Second)
				}
			}()
		}
	}

	<-finished

	return
}
