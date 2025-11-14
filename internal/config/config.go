package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ScicatUrl                string            `yaml:"scicatUrl"`
	FacilityCollectionIDs    map[string]string `yaml:"facilityCollectionIDs"`
	GlobusScopes             []string          `yaml:"globusScopes"`
	Port                     uint              `yaml:"port"`
	FacilitySrcGroupTemplate string            `yaml:"facilitySrcGroupTemplate"`
	FacilityDstGroupTemplate string            `yaml:"facilityDstGroupTemplate"`
	DstPathTemplate          string            `yaml:"destinationPathTemplate"`
	Task                     struct {
		MaxConcurrency int  `yaml:"maxConcurrency"`
		QueueSize      int  `yaml:"queueSize"`
		PollInterval   uint `yaml:"pollInterval"`
	} `yaml:"task"`
}

const confFileName string = "SwissOpenEM/scicat-globus-proxy-conf.yaml"

func ReadConfig() (Config, error) {
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return Config{}, err
	}
	executablePath, err := os.Executable()
	if err != nil {
		return Config{}, err
	}

	primaryConfPath := filepath.Join(filepath.Dir(executablePath), confFileName)
	secondaryConfPath := filepath.Join(userConfigDir, "SwissOpenEM/scicat-globus-proxy", confFileName)

	var conf Config
	f, err := os.ReadFile(primaryConfPath)
	if err == nil {
		err = yaml.Unmarshal(f, &conf)
	} else {
		f, err = os.ReadFile(secondaryConfPath)
		if err != nil {
			return Config{}, fmt.Errorf("no config file found at \"%s\" or \"%s\"", primaryConfPath, secondaryConfPath)
		}
		err = yaml.Unmarshal(f, &conf)
	}

	return conf, err
}
