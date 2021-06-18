package types

import (
	kotsv1beta1 "github.com/replicatedhq/kots/kotskinds/apis/kots/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ApplicationSpec `json:"spec"`
}

type ApplicationSpec struct {
	KOTSApplicationSpec *KOTSApplicationSpec `json:"kots,omitempty"`
}

type KOTSApplicationSpec struct {
	Version        string                        `json:"version,omitempty"`
	App            string                        `json:"app"`
	LicenseID      string                        `json:"licenseID"`
	Endpoint       string                        `json:"endpoint"`
	SkipPreflights *bool                         `json:"skipPreflights,omitempty"`
	Namespace      string                        `json:"namespace,omitempty"`
	ConfigValues   *kotsv1beta1.ConfigValuesSpec `json:"configValues,omitempty"`
}
