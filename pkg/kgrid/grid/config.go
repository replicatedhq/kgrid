package grid

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/pkg/errors"
	"github.com/replicatedhq/kgrid/pkg/kgrid/grid/types"
	"sigs.k8s.io/yaml"
)

var (
	l sync.Mutex
)

func lockConfig() {
	l.Lock()
}

func unlockConfig() {
	l.Unlock()
}

func loadConfig(path string) (*types.GridsConfig, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg := types.GridsConfig{
			GridConfigs: []*types.GridConfig{},
		}
		b, err := yaml.Marshal(cfg)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal empty config")
		}

		if err := os.MkdirAll(filepath.Dir(path), 0744); err != nil {
			return nil, errors.Wrap(err, "failed to create config dir")
		}

		if err := ioutil.WriteFile(path, b, 0644); err != nil {
			return nil, errors.Wrap(err, "failed to write config file")
		}

		return &cfg, nil
	}

	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read config file")
	}

	cfg := types.GridsConfig{}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal config")
	}

	return &cfg, nil
}

func saveConfig(cfg *types.GridsConfig, path string) error {
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return errors.Wrap(err, "failed to marshal config")
	}

	if err := ioutil.WriteFile(path, b, 0644); err != nil {
		return errors.Wrap(err, "failed to write config file")
	}

	return nil
}

func removeGridFromConfig(name string, path string) error {
	lockConfig()
	defer unlockConfig()

	oldCfg, err := loadConfig(path)
	if err != nil {
		return errors.Wrap(err, "failed to load config")
	}

	newCfg := types.GridsConfig{}
	for _, g := range oldCfg.GridConfigs {
		if g.Name == name {
			continue
		}

		newCfg.GridConfigs = append(newCfg.GridConfigs, g)
	}

	if err := saveConfig(&newCfg, path); err != nil {
		return errors.Wrap(err, "failed to save config")
	}

	return nil
}
