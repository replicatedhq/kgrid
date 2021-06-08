package types

import (
	"errors"
	"os"
)

type ValueOrValueFrom struct {
	Value     string     `json:"value,omitempty"`
	ValueFrom *ValueFrom `json:"valueFrom,omitempty"`
}

type ValueFrom struct {
	OSEnv string `json:"osEnv,omitempty"`
}

func (v ValueOrValueFrom) String() (string, error) {
	if v.Value != "" {
		return v.Value, nil
	}

	if v.ValueFrom != nil {
		if v.ValueFrom.OSEnv != "" {
			return os.Getenv(v.ValueFrom.OSEnv), nil
		}
	}

	return "", errors.New("unable to find supported value")
}
