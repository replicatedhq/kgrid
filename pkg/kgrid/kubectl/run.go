package kubectl

import (
	"io/ioutil"
	"os/exec"

	"github.com/pkg/errors"
)

func run(cmd *exec.Cmd) error {
	stdout, stderr, err := runWithOutput(cmd)
	if err != nil {
		if _, ok := errors.Cause(err).(*exec.ExitError); ok {
			// maybe log outputs instead of this "cleverness"
			if len(stderr) > 0 {
				return errors.Wrap(err, string(stderr))
			} else if len(stdout) > 0 {
				return errors.Wrap(err, string(stdout))
			}
		}
		return errors.Wrap(err, "failed to run kubectl")
	}

	return nil
}

func runWithOutput(cmd *exec.Cmd) ([]byte, []byte, error) {
	stdoutReader, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to create stdout reader")
	}
	defer stdoutReader.Close()

	stderrReader, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to create stderr reader")
	}
	defer stderrReader.Close()

	err = cmd.Start()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to start kubectl")
	}

	stdout, _ := ioutil.ReadAll(stdoutReader)
	stderr, _ := ioutil.ReadAll(stderrReader)

	if err := cmd.Wait(); err != nil {
		return stdout, stderr, errors.Wrap(err, "kubectl failed")
	}

	return stdout, stderr, nil
}
