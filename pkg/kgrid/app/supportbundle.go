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
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/pkg/errors"
	"github.com/replicatedhq/kgrid/pkg/kgrid/grid/types"
	"github.com/replicatedhq/kgrid/pkg/kgrid/logger"
)

func generateSupportBundle(c *types.ClusterConfig, log logger.Logger) (string, error) {
	pathToSupportBundleBinary, err := downloadSupportBundleBinary()
	if err != nil {
		return "", errors.Wrap(err, "failed to get support-bundle binary")
	}

	kubeconfigFile, err := ioutil.TempFile("", "supportbundle")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp file")
	}
	defer os.RemoveAll(kubeconfigFile.Name())

	if err := ioutil.WriteFile(kubeconfigFile.Name(), []byte(c.Kubeconfig), 0644); err != nil {
		return "", errors.Wrap(err, "failed to create kubeconfig")
	}

	args := []string{
		"https://kots.io",
		"--kubeconfig", kubeconfigFile.Name(),
		"--interactive=false",
	}

	cmd := exec.Command(pathToSupportBundleBinary, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	cmd.Start()
	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()
	timeout := time.After(5 * time.Minute)

	select {
	case <-timeout:
		cmd.Process.Kill()
		log.Info("timed out generating a support bundle. received std out: %s", stdout.String())
	case err := <-done:
		if err != nil {
			return "", errors.Wrapf(err, "failed to generate a support bundle\nSTDOUT:%s\nSTDERR:%s", stdout.String(), stderr.String())
		}

		log.Info("```%s```", stdout.String())
	}

	type Output struct {
		ArchivePath string `json:"archivePath"`
	}
	output := Output{}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		return "", errors.Wrap(err, "failed to unmarshal output")
	}

	return output.ArchivePath, nil
}

func uploadSupportBundle(path string, log logger.Logger) error {
	if os.Getenv("AWS_S3_BUCKET") == "" {
		log.Info("bucket not specified, not going to upload the support bundle.")
		return nil
	}
	if os.Getenv("AWS_S3_ACCESS_KEY_ID") == "" || os.Getenv("AWS_S3_SECRET_ACCESS_KEY") == "" {
		log.Info("missing aws credentials, not going to upload the support bundle.")
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return errors.Wrap(err, "failed to open archive file")
	}
	defer f.Close()

	newSession := awssession.New(getS3Config())
	s3Client := s3.New(newSession)

	key := fmt.Sprintf("%s.tar.gz", os.Getenv("TEST_ID"))
	if prefix := os.Getenv("RUN_ID"); prefix != "" {
		key = fmt.Sprintf("%s/%s", prefix, key)
	}

	_, err = s3Client.PutObject(&s3.PutObjectInput{
		Body:   f,
		Bucket: aws.String(os.Getenv("AWS_S3_BUCKET")),
		Key:    aws.String(key),
	})
	if err != nil {
		return errors.Wrap(err, "failed to upload to s3")
	}

	return nil
}

func downloadSupportBundleBinary() (string, error) {
	url := fmt.Sprintf("https://github.com/replicatedhq/troubleshoot/releases/latest/download/support-bundle_linux_amd64.tar.gz")
	resp, err := http.Get(url)
	if err != nil {
		return "", errors.Wrapf(err, "failed to http get support-bundle binary from %s", url)
	}
	if resp.StatusCode != http.StatusOK {
		return "", errors.Errorf("failed to download from %s, unexpected status code %d", url, resp.StatusCode)
	}

	archiveFile, err := ioutil.TempFile("", "supportbundle")
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
		if name == "support-bundle" {
			tmpDir, err := ioutil.TempDir("", "supportbundle")
			if err != nil {
				return "", errors.Wrap(err, "failed to create tmp dir")
			}

			binaryPath := filepath.Join(tmpDir, "support-bundle")
			f, err := os.Create(binaryPath)
			if err != nil {
				return "", errors.Wrap(err, "failed to os create file")
			}
			defer f.Close()

			if _, err := io.Copy(f, tarReader); err != nil {
				return "", errors.Wrap(err, "failed to copy support-bundle binary")
			}
			if err := os.Chmod(binaryPath, 0777); err != nil {
				return "", errors.Wrap(err, "faild to chmod")
			}

			return binaryPath, nil
		}
	}

	return "", errors.New("support-bundle binary not found in release")
}

func getS3Config() *aws.Config {
	accessKeyID := os.Getenv("AWS_S3_ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("AWS_S3_SECRET_ACCESS_KEY")

	var creds *credentials.Credentials
	if accessKeyID != "" && secretAccessKey != "" {
		creds = credentials.NewStaticCredentials(accessKeyID, secretAccessKey, "")
	}

	s3Config := &aws.Config{
		Credentials:      creds,
		Region:           aws.String(os.Getenv("AWS_S3_REGION")),
		DisableSSL:       aws.Bool(false),
		S3ForcePathStyle: aws.Bool(false),
	}

	return s3Config
}
