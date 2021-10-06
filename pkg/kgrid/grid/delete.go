package grid

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/pkg/errors"
	"github.com/replicatedhq/kgrid/pkg/kgrid/grid/types"
	"github.com/replicatedhq/kgrid/pkg/kgrid/logger"
)

func Delete(configFilePath string, g *types.Grid, log logger.Logger) error {
	gridConfigs, err := List(configFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to list grid configs")
	}

	wg := sync.WaitGroup{}
	for _, gridConfig := range gridConfigs {
		for _, clusterConfig := range gridConfig.ClusterConfigs {
			for _, cluster := range g.Spec.Clusters {
				if cluster.EKS == nil && cluster.EKS.NewCluster == nil {
					continue
				}

				if cluster.EKS.NewCluster.Name == "" {
					return errors.New("cluster has no name")
				}
				if clusterConfig.Name != cluster.EKS.NewCluster.Name {
					continue
				}

				wg.Add(1)
				go func(config *types.ClusterConfig, cluster *types.ClusterSpec) {
					defer wg.Done()

					err := deleteCluster(config, cluster, log)
					if err != nil {
						fmt.Printf("cluster %s delete failed with error: %v\n", config.Name, err)
					}
				}(clusterConfig, cluster)
			}
		}
	}

	wg.Wait()

	if err := removeGridFromConfig(g.Name, configFilePath); err != nil {
		return errors.Wrap(err, "failed to remove grid from config")
	}

	return nil
}

func deleteCluster(c *types.ClusterConfig, cluster *types.ClusterSpec, log logger.Logger) error {
	if c.Provider == "aws" {
		return deleteNewEKSCluster(c, cluster.EKS, log)
	}

	return nil
}

func deleteNewEKSCluster(c *types.ClusterConfig, cluster *types.EKSSpec, log logger.Logger) error {
	if cluster.NewCluster == nil {
		return errors.New("cluster spec is nil")
	}

	log.Info("Deleting EKS cluster %s", cluster.NewCluster.Name)

	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(c.Region))
	if err != nil {
		return errors.Wrap(err, "failed to load aws config")
	}

	accessKeyID, err := cluster.NewCluster.AccessKeyID.String()
	if err != nil {
		return errors.Wrap(err, "failed to get access key id")
	}
	secretAccessKey, err := cluster.NewCluster.SecretAccessKey.String()
	if err != nil {
		return errors.Wrap(err, "failed to get secret access key")
	}

	cfg.Credentials = credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")

	log.Info("Deleting node group for EKS cluster (this may take a few minutes)")
	err = deleteEKSNodeGroup(cfg, cluster.NewCluster.Name, cluster.NewCluster.Name)
	if err != nil {
		return errors.Wrap(err, "failed to delete node group")
	}

	err = waitEKSNodeGroupGone(cfg, cluster.NewCluster.Name, cluster.NewCluster.Name)
	if err != nil {
		return errors.Wrap(err, "failed to wait for node group delete")
	}

	log.Info("Deleting EKS cluster")
	err = deleteEKSCluster(cfg, cluster.NewCluster.Name)
	if err != nil {
		return errors.Wrap(err, "failed to delete cluster")
	}

	return nil
}
