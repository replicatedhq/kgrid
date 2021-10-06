package cli

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	extensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extensionsscheme "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	"k8s.io/client-go/kubernetes/scheme"
)

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "schemagen",
		Short:        "Generate openapischemas for the kinds in this project",
		SilenceUsage: true,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			return generateSchemas(v)
		},
	}

	cobra.OnInitialize(initConfig)

	cmd.Flags().String("output-dir", "./schemas", "directory to save the schemas in")

	viper.BindPFlags(cmd.Flags())

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	return cmd
}

func InitAndExecute() {
	if err := RootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func initConfig() {
	viper.SetEnvPrefix("KGRID")
	viper.AutomaticEnv()
}

func generateSchemas(v *viper.Viper) error {
	// we generate schemas from the config/crds in the root of this project
	// those crds can be created from controller-gen or by running `make openapischema`

	workdir, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "failed to get workdir")
	}

	files := []struct {
		inFilename  string
		outFilename string
	}{
		{
			"kgrid.replicated.com_grids.yaml",
			"grid-kgrid-v1alpha1.json",
		},
		{
			"kgrid.replicated.com_applications.yaml",
			"application-kgrid-v1alpha1.json",
		},
		{
			"kgrid.replicated.com_versions.yaml",
			"version-kgrid-v1alpha1.json",
		},
		{
			"kgrid.replicated.com_results.yaml",
			"results-kgrid-v1alpha1.json",
		},
	}

	for _, file := range files {
		contents, err := ioutil.ReadFile(filepath.Join(workdir, "config", "crd", "bases", file.inFilename))
		if err != nil {
			return errors.Wrapf(err, "failed to read crd from %s", file.inFilename)
		}
		if err := generateSchemaFromCRD(contents, filepath.Join(workdir, v.GetString("output-dir"), file.outFilename)); err != nil {
			return errors.Wrapf(err, "failed to write crd schema to %s", file.outFilename)
		}
	}

	return nil
}

func generateSchemaFromCRD(crd []byte, outfile string) error {
	extensionsscheme.AddToScheme(scheme.Scheme)
	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode(crd, nil, nil)
	if err != nil {
		return errors.Wrap(err, "failed to decode crd")
	}

	customResourceDefinition := obj.(*extensionsv1.CustomResourceDefinition)

	// There's only v1alpha1 version, which is the one and only element in the array
	b, err := json.MarshalIndent(customResourceDefinition.Spec.Versions[0].Schema.OpenAPIV3Schema, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal json")
	}

	_, err = os.Stat(outfile)
	if err == nil {
		if err := os.Remove(outfile); err != nil {
			return errors.Wrap(err, "failed to remove file")
		}
	}

	d, _ := path.Split(outfile)
	_, err = os.Stat(d)
	if os.IsNotExist(err) {
		if err = os.MkdirAll(d, 0755); err != nil {
			return errors.Wrap(err, "failed to mkdir")
		}
	}

	err = ioutil.WriteFile(outfile, b, 0644)
	if err != nil {
		return errors.Wrap(err, "failed to write file")
	}

	return nil
}
