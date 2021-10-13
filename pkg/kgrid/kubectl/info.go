package kubectl

import (
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"github.com/replicatedhq/kgrid/pkg/kgrid/grid/types"
)

func CheckAPIServer(c *types.ClusterConfig) error {
	kubeconfigFile, err := ioutil.TempFile("", "kubectl")
	if err != nil {
		return errors.Wrap(err, "failed to create temp file")
	}
	defer os.RemoveAll(kubeconfigFile.Name())

	if err := ioutil.WriteFile(kubeconfigFile.Name(), []byte(c.Kubeconfig), 0644); err != nil {
		return errors.Wrap(err, "failed to create kubeconfig")
	}

	args := []string{
		"--kubeconfig", kubeconfigFile.Name(),
		"cluster-info",
	}

	cmd := exec.Command("kubectl", args...)

	stdout, stderr, err := runWithOutput(cmd)
	if err != nil {
		return errors.Wrapf(err, "failed to run kubectl command: %s", stderr)
	}

	// output contains terminal control characters in the middle of the phrase
	if strings.Contains(string(stdout), "Kubernetes control plane") &&
		strings.Contains(string(stdout), "is running") {
		return nil
	}

	return errors.New(string(stderr))
}
