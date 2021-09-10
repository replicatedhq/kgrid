package types

import (
	"crypto/md5"
	"fmt"
)

type Grid struct {
	Name string   `json:"name"`
	Spec GridSpec `json:"spec"`
}

type GridSpec struct {
	Clusters []*ClusterSpec `json:"clusters"`
}

type ClusterSpec struct {
	Logger LoggerSpec `json:"logger"`
	EKS    *EKSSpec   `json:"eks,omitempty"`
}

type EKSSpec struct {
	ExistingCluster *EKSExistingClusterSpec `json:"existingCluster,omitempty"`
	NewCluster      *EKSNewClusterSpec      `json:"newCluster,omitempty"`
}

type EKSExistingClusterSpec struct {
	AccessKeyID     ValueOrValueFrom `json:"accessKeyId"`
	SecretAccessKey ValueOrValueFrom `json:"secretAccessKey"`
	ClusterName     string           `json:"clusterName"`
	Region          string           `json:"region"`
}

type EKSNewClusterSpec struct {
	Description     string           `json:"description,omitempty"`
	Version         string           `json:"version,omitempty"`
	AccessKeyID     ValueOrValueFrom `json:"accessKeyId"`
	SecretAccessKey ValueOrValueFrom `json:"secretAccessKey"`
	Region          string           `json:"region"`
}

type LoggerSpec struct {
	Slack *SlackLoggerSpec `json:"slack,omitempty"`
}

type SlackLoggerSpec struct {
	Token   ValueOrValueFrom `json:"token,omitempty"`
	Channel string           `json:"channel,omitempty"`
}

func (c EKSNewClusterSpec) GetDeterministicClusterName() string {
	return fmt.Sprintf("grid-%x", md5.Sum([]byte(fmt.Sprintf("%s-%s-%s", c.Description, c.Region, c.Version))))
}

func (c ClusterSpec) GetNameForLogging() string {
	if c.EKS != nil {
		if c.EKS.ExistingCluster != nil {
			return c.EKS.ExistingCluster.ClusterName
		}
		if c.EKS.NewCluster != nil {
			return c.EKS.NewCluster.Description
		}
	}

	return ""
}
