package types

import (
	"crypto/md5"
	"fmt"
)

type GridsConfig struct {
	GridConfigs []*GridConfig `json:"grids,omitempty"`
}

type GridConfig struct {
	Name           string           `json:"name"`
	ClusterConfigs []*ClusterConfig `json:"clusters,omitempty"`
}

type ClusterConfig struct {
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	IsExisting  bool   `json:"isExisting"`
	Region      string `json:"region"`
	Kubeconfig  string `json:"kubeconfig,omitempty"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version,omitempty"`
}

func (c ClusterConfig) GetDeterministicClusterName() string {
	return fmt.Sprintf("grid-%x", md5.Sum([]byte(fmt.Sprintf("%s-%s-%s", c.Description, c.Region, c.Version))))
}
