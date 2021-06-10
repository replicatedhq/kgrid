package kubectl

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/pkg/errors"
	"github.com/replicatedhq/kgrid/pkg/kgrid/grid/types"
)

type Metadata struct {
	Name string `json:"name"`
}

type Conditions struct {
	Message string `json:"message"`
	Reason  string `json:"reason"`
	Status  string `json:"status"`
	Type    string `json:"type"`
}

type Node struct {
	Metadata Metadata   `json:"metadata"`
	Status   NodeStatus `json:"status"`
}

type NodeStatus struct {
	Conditions []Conditions `json:"conditions"`
}

type Nodes struct {
	Items []Node `json:"items"`
}

func GetNodes(c *types.ClusterConfig) (Nodes, error) {
	kubeconfigFile, err := ioutil.TempFile("", "kubectl")
	if err != nil {
		return Nodes{}, errors.Wrap(err, "failed to create temp file")
	}
	defer os.RemoveAll(kubeconfigFile.Name())

	if err := ioutil.WriteFile(kubeconfigFile.Name(), []byte(c.Kubeconfig), 0644); err != nil {
		return Nodes{}, errors.Wrap(err, "failed to create kubeconfig")
	}

	args := []string{
		"--kubeconfig", kubeconfigFile.Name(),
		"get", "nodes",
		"-o", "json",
	}

	cmd := exec.Command("kubectl", args...)

	stdout, stderr, err := runWithOutput(cmd)
	if err != nil {
		return Nodes{}, errors.Wrapf(err, "failed to run kubectl command: %s", stderr)
	}

	nodes := Nodes{}
	err = json.Unmarshal(stdout, &nodes)
	if err != nil {
		return Nodes{}, errors.Wrap(err, "failed to unmarshal nodes")
	}

	return nodes, nil
}
