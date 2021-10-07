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

type TestResultStatus string

const (
	TestResultPass    TestResultStatus = "Pass"
	TestResultFail    TestResultStatus = "Fail"
	TestResultUnknown TestResultStatus = "Unknown"
	TestResultPending TestResultStatus = "Pending"
)

type Test struct {
	ID     string           `json:"id"`
	Result TestResultStatus `json:"result,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+genclient
//+k8s:openapi-gen=true

// Outcome is the Schema for the results API
type Outcome struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Tests []Test `json:"tests"`
}

//+kubebuilder:object:root=true

// OutcomeList contains a list of Results
type OutcomeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Outcome `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Outcome{}, &OutcomeList{})
}
