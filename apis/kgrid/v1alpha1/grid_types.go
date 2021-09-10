/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type Cluster struct {
	Name   string  `json:"name"`
	EKS    *EKS    `json:"eks,omitempty"`
	Logger *Logger `json:"logger,omitempty"`
}

type EKS struct {
	Region          string           `json:"region"`
	Version         string           `json:"version,omitempty"`
	Create          bool             `json:"create"`
	AaccessKeyID    ValueOrValueFrom `json:"accessKeyId"`
	SecretAccessKey ValueOrValueFrom `json:"secretAccessKey"`
}

type SlackLogger struct {
	Token   ValueOrValueFrom `json:"token,omitempty"`
	Channel string           `json:"channel,omitempty"`
}

type Logger struct {
	Slack *SlackLogger `json:"slack,omitempty"`
}

// GridSpec defines the desired state of Grid
type GridSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Clusters []Cluster `json:"clusters,omitempty"`
}

// GridStatus defines the observed state of Grid
type GridStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+genclient
//+k8s:openapi-gen=true

// Grid is the Schema for the grids API
type Grid struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GridSpec   `json:"spec,omitempty"`
	Status GridStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GridList contains a list of Grid
type GridList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Grid `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Grid{}, &GridList{})
}
