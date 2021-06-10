package kubectl

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/pkg/errors"
	"github.com/replicatedhq/kgrid/pkg/kgrid/grid/types"
)

func Apply(c *types.ClusterConfig, yamlDoc string) error {
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
		"apply",
		"-f", "-",
	}

	cmd := exec.Command("kubectl", args...)
	cmd.Stdin = bytes.NewReader([]byte(yamlDoc))

	err = run(cmd)
	if err != nil {
		return errors.Wrap(err, "failed to run kubectl command")
	}

	return nil
}
