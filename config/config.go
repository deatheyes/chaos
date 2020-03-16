package config

import (
	"io/ioutil"

	"gopkg.in/yaml.v3"
)

// SystemTap set the env for systemtap
type SystemTap struct {
	ScriptDir  string `yaml:"script_dir"` // dir of the systemtap script
	IOProbeDir string `yaml:"probe_dir"`  // dir to probe
	PidFile    string `yaml:"pidfile"`    // file to get pid
}

// Service set the env of service
type Service struct {
	Start string `yaml:"start"` // command to start the service
	Stop  string `yaml:"stop"`  // command to stop the service
}

// ControllerConfig is the base configuration of controller
type ControllerConfig struct {
	SystemTap  *SystemTap          `yaml:"systemtap"` // systemtap configuration
	ServiceMap map[string]*Service `yaml:"service"`   // service configuration
}

func (c *ControllerConfig) String() string {
	content, _ := yaml.Marshal(c)
	return string(content)
}

// LoadControllerConfigFromFile create controller configuration from file
func LoadControllerConfigFromFile(file string) (*ControllerConfig, error) {
	content, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	c := &ControllerConfig{}
	if err := yaml.Unmarshal(content, c); err != nil {
		return nil, err
	}
	return c, nil
}
