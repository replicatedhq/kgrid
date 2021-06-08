package cluster

import (
	"context"
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
	"github.com/replicatedhq/kgrid/pkg/kgrid/grid/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func ListNamespaces(clusterConfig *types.ClusterConfig) (*corev1.NamespaceList, error) {
	tmp, err := ioutil.TempFile("", "grid")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temp file")
	}
	defer os.RemoveAll(tmp.Name())

	if err := ioutil.WriteFile(tmp.Name(), []byte(clusterConfig.Kubeconfig), 0644); err != nil {
		return nil, errors.Wrap(err, "failed to create kubeconfig file")
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", tmp.Name())
	if err != nil {
		return nil, errors.Wrap(err, "failed to build client-go config")
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create clientset")
	}

	namespaces, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list namespapces")
	}

	return namespaces, nil
}
