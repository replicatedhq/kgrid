package app

import (
	"encoding/json"
	"reflect"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/replicatedhq/kgrid/pkg/kgrid/grid/types"
	"github.com/replicatedhq/kgrid/pkg/kgrid/logger"
)

type DeployStatus string

const (
	DeployInProgress DeployStatus = "in_progress"
	DeployFailed     DeployStatus = "failed"
	DeploySucceeded  DeployStatus = "succeeded"
)

func Deploy(g *types.GridConfig, a *types.Application, log logger.Logger) (finalError error) {
	deployStatuses := map[int]DeployStatus{}
	deployChans := make([]chan string, len(g.ClusterConfigs))
	for i := range g.ClusterConfigs {
		deployStatuses[i] = DeployInProgress
		deployChans[i] = make(chan string)
	}

	finished := make(chan bool)
	go func() {
		cases := make([]reflect.SelectCase, len(deployChans))
		for i, ch := range deployChans {
			cases[i] = reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(ch),
			}
		}

		wg := sync.WaitGroup{}

		for {
			i, deployErr, ok := reflect.Select(cases)
			if ok {
				if deployErr.String() != "" {
					finalError = errors.New("application failed to deploy")
					deployStatuses[i] = DeployFailed

					log.Info("deploy to cluster %s failed with error: %s\n", g.ClusterConfigs[i].Name, deployErr.String())
					log.Info("generating support bundle for cluster %s\n", g.ClusterConfigs[i].Name)

					wg.Add(1)
					go func(clusterName string) {
						defer wg.Done()
						path, err := generateSupportBundle(g.ClusterConfigs[i], log)
						if err != nil {
							log.Info("failed to generate a support bundle for cluster %s, %v", clusterName, err)
							return
						}
						if err := uploadSupportBundle(path, log); err != nil {
							log.Info("failed to upload support bundle for cluster %s: %v", clusterName, err)
							return
						}
					}(g.ClusterConfigs[i].Name)
				} else {
					deployStatuses[i] = DeploySucceeded
				}
			}

			allCompleted := true
			for _, v := range deployStatuses {
				if v != DeploySucceeded && v != DeployFailed {
					allCompleted = false
				}
			}

			if allCompleted {
				wg.Wait()
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
					deployChans[i] <- errors.Wrapf(err, "failed to get kots %s binary", a.Spec.KOTSApplicationSpec.Version).Error()
					return
				}

				err = deployKOTSApplication(c, a.Spec.KOTSApplicationSpec, pathToKOTSBinary, log)
				if err != nil {
					deployChans[i] <- err.Error()
					return
				}

				waitUntil := time.Now().Add(5 * time.Minute)
				for {
					appStatus, err := getKOTSApplicationStatus(c, a.Spec.KOTSApplicationSpec, pathToKOTSBinary, log)
					if err != nil {
						deployChans[i] <- err.Error()
						return
					}

					statusString, _ := json.MarshalIndent(appStatus, "", "  ")
					log.Info("```%s```", statusString)
					if appStatus.AppStatus.State == "ready" {
						deployChans[i] <- ""
						return
					}

					if time.Now().After(waitUntil) {
						deployChans[i] <- "timed out waiting for app ready status"
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
