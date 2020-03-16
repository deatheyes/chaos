package config

import (
	"fmt"
	"io/ioutil"
	"path"
	"strings"

	"github.com/deatheyes/chaos/util"
	"github.com/golang/glog"
	"gopkg.in/yaml.v2"
)

// Strategy specifies how to run tasks
type Strategy struct {
	Selector    string `yaml:"selector"`     // node selector, randomly select currently
	TickerCycle int    `yaml:"ticker_cycle"` // ticker Cycle
	TickerLast  int    `yaml:"ticker_last"`  // ticker duration
	Timeout     int    `yaml:"timeout"`      // timeout configuration
	Limit       int    `yaml:"limit"`        // limitation of nodes selected
}

// ToSelector create a Selector form Strategy
func (s *Strategy) ToSelector() Selector {
	selector := make(Selector)
	segments := strings.Split(s.Selector, ",")
	for _, v := range segments {
		segs := strings.Split(v, "=")
		if len(segs) == 2 {
			selector[segs[0]] = segs[1]
		}
	}
	return selector
}

// Task configuration
type Task struct {
	Type      string            `yaml:"type"`      // type of task, eg. nemesis timer
	Name      string            `yaml:"name"`      // unique name of the task
	Arguments map[string]string `yaml:"arguments"` // arguments to run the task
	Strategy  Strategy          `yaml:"strategy"`  // rule to select node
	Requires  []string          `yaml:"requires"`  // upstream of this task
	ID        string            `yaml:"-"`
}

// Stage is a group of tasks within a closure
type Stage struct {
	Name            string             `yaml:"name"`
	Description     string             `yaml:"description"`
	TaskList        []*Task            `yaml:"tasks"`
	TaskDict        map[string]*Task   `yaml:"-"`
	TaskRequireDict map[string][]*Task `yaml:"-"`
}

func (s *Stage) build() error {
	// build inverted indices as helper
	for _, v := range s.TaskList {
		fmt.Println(v.Name)
		if _, ok := s.TaskDict[v.Name]; ok {
			return fmt.Errorf("build failed duplicated task name %s", v.Name)
		}
		s.TaskDict[v.Name] = v
		for _, r := range v.Requires {
			s.TaskRequireDict[r] = append(s.TaskRequireDict[r], v)
		}
	}
	return nil
}

// LoadStageFromFile create a stage configuration from file
func LoadStageFromFile(file string) (*Stage, error) {
	content, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	s := &Stage{TaskDict: make(map[string]*Task), TaskRequireDict: make(map[string][]*Task)}
	if err := yaml.Unmarshal(content, s); err != nil {
		return nil, err
	}

	if err := s.build(); err != nil {
		return nil, err
	}
	return s, nil
}

const (
	stageFileSuffix = ".yaml"
)

// LoadStageFromDir load all stage from data dir
func LoadStageFromDir(dir string) ([]*Stage, error) {
	ret := []*Stage{}
	list, err := util.ListAllFileByName(dir, stageFileSuffix)
	if err != nil {
		return ret, nil
	}

	for _, item := range list {
		p := path.Join(dir, item)
		s, err := LoadStageFromFile(p)
		if err != nil {
			glog.Warningf("[CONFIG] load stage form file %s failed:%v", p, err)
		} else {
			ret = append(ret, s)
		}
	}
	return ret, nil
}

// StageToString convert a Stage struct to string for debugging
func StageToString(s *Stage) string {
	content, _ := yaml.Marshal(s)
	return string(content)
}
