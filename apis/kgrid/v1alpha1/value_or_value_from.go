package v1alpha1

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/replicatedhq/kgrid/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type ValueOrValueFrom struct {
	Value     string     `json:"value,omitempty" yaml:"value,omitempty"`
	ValueFrom *ValueFrom `json:"valueFrom,omitempty" yaml:"valueFrom,omitempty"`
}

// IsEmpty returns true if there is not a value in value and valuefrom
func (v *ValueOrValueFrom) IsEmpty() bool {
	if v.Value != "" {
		return false
	}

	if v.ValueFrom != nil {
		return false
	}

	return true
}

func (v ValueOrValueFrom) String(ctx context.Context, namespace string) (string, error) {
	if v.Value != "" {
		return v.Value, nil
	}

	if v.ValueFrom.SecretKeyRef != nil {
		val, err := v.getValueFromSecret(ctx, namespace)
		return val, errors.Wrap(err, "failed ot get value from secret")
	}

	return "", nil
}

func (v ValueOrValueFrom) getValueFromSecret(ctx context.Context, namespace string) (string, error) {
	cfg, err := config.GetRESTConfig()
	if err != nil {
		return "", errors.Wrap(err, "failed to get config")
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return "", errors.Wrap(err, "failed to get clientset")
	}

	secretKeyRefName := v.ValueFrom.SecretKeyRef.Name
	secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, secretKeyRefName, metav1.GetOptions{})
	if err != nil {
		return "", errors.Wrap(err, "failed to get secret")
	}

	keyName := v.ValueFrom.SecretKeyRef.Key
	keyData, ok := secret.Data[keyName]
	if !ok {
		return "", fmt.Errorf("expected Secret \"%s\" to contain key \"%s\"", secretKeyRefName, keyName)
	}
	return string(keyData), nil
}
