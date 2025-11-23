package config

import (
	"fmt"
	"os"
	"path/filepath"

	util "github.com/SwissOpenEM/scicat-globus-proxy/internal/util"
	"github.com/goccy/go-yaml"
)

type Config struct {
	ScicatUrl  string           `yaml:"scicatUrl"`
	Facilities []FacilityConfig `yaml:"facilities"`
	Port       uint             `yaml:"port"`
	Task       TaskConfig       `yaml:"task,omitempty"`
}

type TaskConfig struct {
	MaxConcurrency int  `yaml:"maxConcurrency,omitempty"`
	QueueSize      int  `yaml:"queueSize,omitempty"`
	PollInterval   uint `yaml:"pollInterval,omitempty"`
}

// Modify a TaskConfig by overridding any non-zero fields specified in the argument
func (conf *TaskConfig) Merge(overrides *TaskConfig) *TaskConfig {
	if conf == nil || overrides == nil {
		return conf
	}

	if overrides.MaxConcurrency != 0 {
		conf.MaxConcurrency = overrides.MaxConcurrency
	}
	if overrides.QueueSize != 0 {
		conf.QueueSize = overrides.QueueSize
	}
	if overrides.PollInterval != 0 {
		conf.PollInterval = overrides.PollInterval
	}
	return conf
}

// Construct a FacilityConfig with default values
func NewTaskConfig() TaskConfig {
	return TaskConfig{
		MaxConcurrency: 10,
		QueueSize:      0,
		PollInterval:   10,
	}
}

type FacilityDirection string

const (
	DirectionSource      FacilityDirection = "SRC"
	DirectionDestination FacilityDirection = "DST"
	DirectionBoth        FacilityDirection = "BOTH"
)

type FacilityConfig struct {
	Name               string            `yaml:"name"`
	Collection         string            `yaml:"collection"`
	Scopes             []string          `yaml:"scopes,omitempty"`
	AccessPath         string            `yaml:"accessPath,omitempty"`
	AccessValue        string            `yaml:"accessValue,omitempty"`
	Direction          FacilityDirection `yaml:"direction,omitempty"`
	SourcePath         string            `yaml:"sourcePath,omitempty"`
	DestinationPath    string            `yaml:"destinationPath,omitempty"`
	CollectionRootPath string            `yaml:"collectionRootPath,omitempty"`
}

// Construct a FacilityConfig with default values
func NewFacilityConfig() *FacilityConfig {
	return &FacilityConfig{
		Collection: "",
		Scopes: []string{
			"urn:globus:auth:scope:transfer.api.globus.org:all[*https://auth.globus.org/scopes/{{.Collection}}/data_access]",
		},
		AccessPath:         "profile.accessGroups",
		AccessValue:        "{{ .Name }}",
		Direction:          DirectionBoth,
		SourcePath:         "/{{ .RelativeSourceFolder }}",
		DestinationPath:    "/{{ .RelativeSourceFolder }}",
		CollectionRootPath: "/",
	}
}

// Modify a config by overridding any non-zero fields specified in the argument
func (base *FacilityConfig) Merge(overrides *FacilityConfig) *FacilityConfig {
	if base == nil || overrides == nil {
		return base
	}

	if overrides.Name != "" {
		base.Name = overrides.Name
	}
	if overrides.Collection != "" {
		base.Collection = overrides.Collection
	}
	// Copy the new scope slice, rather than appending
	if len(overrides.Scopes) > 0 {
		base.Scopes = make([]string, len(overrides.Scopes))
		copy(base.Scopes, overrides.Scopes)
	}
	if overrides.AccessPath != "" {
		base.AccessPath = overrides.AccessPath
	}
	if overrides.AccessValue != "" {
		base.AccessValue = overrides.AccessValue
	}
	if overrides.Direction != "" {
		base.Direction = overrides.Direction
	}
	if overrides.SourcePath != "" {
		base.SourcePath = overrides.SourcePath
	}
	if overrides.DestinationPath != "" {
		base.DestinationPath = overrides.DestinationPath
	}
	if overrides.CollectionRootPath != "" {
		base.CollectionRootPath = overrides.CollectionRootPath
	}
	return base
}

const confFileName string = "scicat-globus-proxy-config.yaml"

// Read the config file
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
	secondaryConfPath := filepath.Join(userConfigDir, "scicat-globus-proxy", confFileName)

	f, err := os.ReadFile(primaryConfPath)
	if err != nil {
		f, err = os.ReadFile(secondaryConfPath)
	}
	if err != nil {
		return Config{}, fmt.Errorf("no config file found at \"%s\" or \"%s\"", primaryConfPath, secondaryConfPath)
	}
	return ReadConfigFromBytes(f)
}

func ReadConfigFromBytes(contents []byte) (Config, error) {
	var conf Config
	if err := yaml.Unmarshal(contents, &conf); err != nil {
		return Config{}, err
	}

	// Set defaults
	task := NewTaskConfig()
	task.Merge(&conf.Task)
	conf.Task = task

	for i, facility := range conf.Facilities {
		merged := NewFacilityConfig()
		merged.Merge(&facility)
		conf.Facilities[i] = *merged
	}

	// Validate required fields
	if conf.ScicatUrl == "" {
		return Config{}, fmt.Errorf("missing ScicatUrl in configuration")
	}
	if len(conf.Facilities) == 0 {
		return Config{}, fmt.Errorf("no facilities defined in configuration")
	}
	for i, facility := range conf.Facilities {
		if facility.Name == "" {
			return Config{}, fmt.Errorf("missing Name for facility %v", i)
		}
	}

	return conf, nil
}

// Variables available to scopes for templating
type globusContext struct {
	Name       string
	Collection string
}

// Get a list of all globus scopes for all facilities
func (conf *Config) GetGlobusScopes() ([]string, error) {
	scopes := make([]string, 0, len(conf.Facilities))
	for _, facility := range conf.Facilities {
		for _, scopeTemplate := range facility.Scopes {
			// Make the facility configuration available in the scope for templating
			context := globusContext{
				Name:       facility.Name,
				Collection: facility.Collection,
			}
			scope, err := util.ExecuteTemplate(scopeTemplate, context)
			if err != nil {
				return nil, fmt.Errorf("error in configuration for facility %s: %w", facility.Name, err)
			}
			scopes = append(scopes, scope)
		}
	}
	return scopes, nil
}
