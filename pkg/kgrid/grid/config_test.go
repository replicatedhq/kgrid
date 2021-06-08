package grid

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/replicatedhq/kgrid/pkg/kgrid/grid/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_loadConfig(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		expected *types.GridsConfig
	}{
		{
			name:     "empty",
			data:     `{}`,
			expected: &types.GridsConfig{},
		},
		{
			name: "single grid",
			data: `grids:
  - clusters:
    - isExisting: true
      kubeconfig: s
      name: s
      provider: aws
      region: us-west-1
    name: s`,
			expected: &types.GridsConfig{
				GridConfigs: []*types.GridConfig{
					{
						Name: "s",
						ClusterConfigs: []*types.ClusterConfig{
							{
								Name:       "s",
								IsExisting: true,
								Kubeconfig: "s",
								Provider:   "aws",
								Region:     "us-west-1",
							},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			// write the data to a tmp file
			tmpFile, err := ioutil.TempFile("", "")
			req.NoError(err)
			defer os.RemoveAll(tmpFile.Name())
			err = ioutil.WriteFile(tmpFile.Name(), []byte(test.data), 0644)
			req.NoError(err)

			actual, err := loadConfig(tmpFile.Name())
			req.NoError(err)

			assert.Equal(t, test.expected, actual)
		})
	}
}
