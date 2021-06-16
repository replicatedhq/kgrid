package app

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gosimple/slug"
	"github.com/pkg/errors"
	"github.com/replicatedhq/kgrid/pkg/kgrid/grid/types"
	kotsv1beta1 "github.com/replicatedhq/kots/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kots/pkg/kotsutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultKOTSVersion = "1.27.0"
	DefaultK6Version   = "0.29.0"
)

type AppStatusResponse struct {
	AppStatus AppStatus `json:"appstatus"`
}

type AppStatus struct {
	AppID          string          `json:"appId"`
	UpdatedAt      time.Time       `json:"updatedAt"`
	ResourceStates []ResourceState `json:"resourceStates"`
	State          string          `json:"state"`
}

type ResourceState struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	State     string `json:"state"`
}

func getAppSlug(c *types.ClusterConfig, kotsAppSpec *types.KOTSApplicationSpec) (string, error) {
	// this is _really_ brittle
	// the KOTS admin console doesn't give us a way to predict the app slug
	// so we've copied the same logic that KOTS uses
	// and it's sort of ok, but definitely is going to screw up

	// let's make kots return a list of apps?
	licenseFilePath, err := downloadKOTSLicense(kotsAppSpec.App, kotsAppSpec.LicenseID)
	if err != nil {
		return "", errors.Wrap(err, "failed to download license")
	}
	defer os.RemoveAll(licenseFilePath)

	license, err := kotsutil.LoadLicenseFromPath(licenseFilePath)
	if err != nil {
		return "", errors.Wrap(err, "failed to load license")
	}

	desiredAppName := strings.Replace(license.Spec.AppSlug, "-", " ", 0)
	titleForSlug := strings.Replace(desiredAppName, ".", "-", 0)
	return slug.Make(titleForSlug), nil
}

// isApplicationReady will return
//  bool1: is the application ready
func isApplicationReady(c *types.ClusterConfig, kotsAppSpec *types.KOTSApplicationSpec) (bool, error) {
	// right now, we just check the status informers

	pathToKOTSBinary, err := downloadKOTSBinary(kotsAppSpec.Version)
	if err != nil {
		return false, errors.Wrapf(err, "failed to get kots %s binary", kotsAppSpec.Version)
	}

	kubeconfigFile, err := ioutil.TempFile("", "kots")
	if err != nil {
		return false, errors.Wrap(err, "failed to create temp file")
	}
	defer os.RemoveAll(kubeconfigFile.Name())
	if err := ioutil.WriteFile(kubeconfigFile.Name(), []byte(c.Kubeconfig), 0644); err != nil {
		return false, errors.Wrap(err, "failed to create kubeconfig")
	}

	namespace := kotsAppSpec.Namespace
	if namespace == "" {
		namespace = kotsAppSpec.App
	}

	appSlug, err := getAppSlug(c, kotsAppSpec)
	if err != nil {
		return false, errors.Wrap(err, "failed to get app slug")
	}

	args := []string{
		"--namespace", namespace,
		"--kubeconfig", kubeconfigFile.Name(),
	}

	allArgs := []string{
		"app-status",
		"-n", namespace,
		appSlug,
	}
	allArgs = append(allArgs, args...)
	cmd := exec.Command(pathToKOTSBinary, allArgs...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	cmd.Start()
	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()
	timeout := time.After(time.Second * 5)

	select {
	case <-timeout:
		cmd.Process.Kill()
		fmt.Printf("timedout waiting for app ready.  received std out: %s\n", stdout.String())
		return false, nil
	case err := <-done:
		if err != nil {
			return false, errors.Wrapf(err, "failed to run kots for status check\nSTDOUT:%s\nSTDERR:%s", stdout.String(), stderr.String())
		}

		appStatusResponse := AppStatusResponse{}
		if err := json.Unmarshal(stdout.Bytes(), &appStatusResponse); err != nil {
			return false, errors.Wrap(err, "faile to parse app status response")
		}

		return appStatusResponse.AppStatus.State == "ready", nil
	}
}

func deployKOTSApplication(c *types.ClusterConfig, kotsAppSpec *types.KOTSApplicationSpec) error {
	// ensure we have the right version of KOTS
	pathToKOTSBinary, err := downloadKOTSBinary(kotsAppSpec.Version)
	if err != nil {
		return errors.Wrapf(err, "failed to get kots %s binary", kotsAppSpec.Version)
	}

	pathToLicense, err := downloadKOTSLicense(kotsAppSpec.App, kotsAppSpec.LicenseID)
	if err != nil {
		return errors.Wrap(err, "failed to get license")
	}
	defer os.RemoveAll(pathToLicense)

	kubeconfigFile, err := ioutil.TempFile("", "kots")
	if err != nil {
		return errors.Wrap(err, "failed to create temp file")
	}
	defer os.RemoveAll(kubeconfigFile.Name())
	if err := ioutil.WriteFile(kubeconfigFile.Name(), []byte(c.Kubeconfig), 0644); err != nil {
		return errors.Wrap(err, "failed to create kubeconfig")
	}

	namespace := kotsAppSpec.Namespace
	if namespace == "" {
		namespace = kotsAppSpec.App
	}

	args := []string{
		"--namespace", namespace,
		"--license-file", pathToLicense,
		"--shared-password", "password",
		"--port-forward=false",
		"--kubeconfig", kubeconfigFile.Name(),
	}

	if kotsAppSpec.ConfigValues != nil {
		configValues := kotsv1beta1.ConfigValues{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "kots.io/v1beta1",
				Kind:       "ConfigValues",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "automated-config-values",
			},
			Spec: *kotsAppSpec.ConfigValues,
		}
		kotsKinds := kotsutil.KotsKinds{
			ConfigValues: &configValues,
		}
		b, err := kotsKinds.Marshal("kots.io", "v1beta1", "ConfigValues")
		if err != nil {
			return errors.Wrap(err, "failed to marshal config values")
		}
		configValuesFile, err := ioutil.TempFile("", "kots")
		if err != nil {
			return errors.Wrap(err, "failed to create temp file")
		}
		defer os.RemoveAll(configValuesFile.Name())
		if err := ioutil.WriteFile(configValuesFile.Name(), []byte(b), 0644); err != nil {
			return errors.Wrap(err, "failed to write config values to file")
		}
		args = append(args, "--config-values")
		args = append(args, configValuesFile.Name())
	}

	if kotsAppSpec.SkipPreflights != nil && *kotsAppSpec.SkipPreflights {
		args = append(args, "--skip-preflights")
	}

	allArgs := []string{
		"install",
		kotsAppSpec.App,
	}
	allArgs = append(allArgs, args...)

	cmd := exec.Command(pathToKOTSBinary, allArgs...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	cmd.Start()
	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()
	timeout := time.After(10 * time.Minute) // TODO: app deploy already has a timeout

	select {
	case <-timeout:
		cmd.Process.Kill()
		fmt.Printf("timeoud out deploying app.  received std out: %s\n", stdout.String())
	case err := <-done:
		if err != nil {
			return errors.Wrapf(err, "failed to run kots for deploy\nSTDOUT:%s\nSTDERR:%s", stdout.String(), stderr.String())
		}

		fmt.Printf("%s\n", stdout.String())
	}

	return nil
}

// the caller is responsible for deleting the file
func downloadKOTSLicense(appSlug string, licenseID string) (string, error) {
	url := fmt.Sprintf("https://replicated.app/license/%s", appSlug)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", errors.Wrap(err, "failed to create new request")
	}

	req.SetBasicAuth(licenseID, licenseID)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "failed to execute request")
	}
	if resp.StatusCode != http.StatusOK {
		return "", errors.Errorf("unexpected status code downloading license: %d", resp.StatusCode)
	}

	archiveFile, err := ioutil.TempFile("", "kots")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp archive file")
	}

	_, err = io.Copy(archiveFile, resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "Failed to copy file")
	}

	archiveFile.Close()
	return archiveFile.Name(), nil
}

func downloadKOTSBinary(version string) (string, error) {
	if version == "" {
		version = DefaultKOTSVersion
	}

	url := fmt.Sprintf("https://github.com/replicatedhq/kots/releases/download/v%s/kots_linux_amd64.tar.gz", version)
	resp, err := http.Get(url)
	if err != nil {
		return "", errors.Wrapf(err, "failed to http get kots from %s", url)
	}
	if resp.StatusCode != http.StatusOK {
		return "", errors.Errorf("failed to download from %s, unexpected status code %d", url, resp.StatusCode)
	}

	archiveFile, err := ioutil.TempFile("", "kots")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp archive file")
	}
	defer os.RemoveAll(archiveFile.Name())

	_, err = io.Copy(archiveFile, resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to save archive file")
	}

	if _, err := archiveFile.Seek(0, 0); err != nil {
		return "", errors.Wrap(err, "failed to seek")
	}

	gzf, err := gzip.NewReader(archiveFile)
	if err != nil {
		return "", errors.Wrap(err, "failed to create gzip reader")
	}

	tarReader := tar.NewReader(gzf)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return "", errors.Wrap(err, "failed to read next file")
		}

		name := header.Name
		if name == "kots" {
			tmpDir, err := ioutil.TempDir("", "kots")
			if err != nil {
				return "", errors.Wrap(err, "failed to create tmp dir")
			}

			binaryPath := filepath.Join(tmpDir, "kots")
			f, err := os.Create(binaryPath)
			if err != nil {
				return "", errors.Wrap(err, "failed to os create file")
			}
			defer f.Close()
			if _, err := io.Copy(f, tarReader); err != nil {
				return "", errors.Wrap(err, "failed to copy kots binary")
			}
			if err := os.Chmod(binaryPath, 0777); err != nil {
				return "", errors.Wrap(err, "faild to chmod")
			}

			return binaryPath, nil
		}
	}

	return "", errors.New("kots binary not found in release")
}
