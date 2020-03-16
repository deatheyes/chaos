package control

import (
	"context"
	"sync"

	"github.com/golang/glog"
)

// Stage is consisted by a group of tasks with the same head.
// TODO Stage DAG
type Stage struct {
	Name       string                 // stage name
	TaskDict   map[string]Task        // nodes of DAG
	RunnerDict map[string]*TaskRunner // nodes of Runner
	Notify     chan Task              // notify to proceed the DAG
	Cancel     func()                 // call back to terminate this stage
	Running    bool                   // is running ?
	sync.Mutex
}

// NewStage create a new stage
func NewStage(name string) *Stage {
	return &Stage{
		Name:       name,
		TaskDict:   make(map[string]Task),
		RunnerDict: make(map[string]*TaskRunner),
		Notify:     make(chan Task),
		Running:    false,
	}
}

// Run starts the stage
func (s *Stage) Run(ctx context.Context) error {
	glog.Infof("[STAGE][%s] run", s.Name)
	go func() {
		s.Notify <- s.TaskDict[Head]
	}()

	for {
		select {
		case t := <-s.Notify:
			glog.V(8).Infof("[STAGE][%s] get notify from task %s %s", s.Name, t.GetMeta().Name, t.GetMeta().ID)
			downstream := t.GetMeta().Downstream
			for _, v := range downstream {
				// create runner if necessary
				// boradcast to downstream
				if _, ok := s.RunnerDict[v.GetMeta().ID]; !ok {
					glog.V(8).Infof("[STAGE][%s] create runner for task %s %s", s.Name, v.GetMeta().Name, v.GetMeta().ID)
					runner := NewTaskRunner(v, s.Notify)
					s.RunnerDict[v.GetMeta().ID] = runner
					go runner.Run(ctx)
				}

				go func(r *TaskRunner) {
					r.In <- t
				}(s.RunnerDict[v.GetMeta().ID])
			}
		case <-ctx.Done():
			glog.Infof("[STAGE][%s] canceled", s.Name)
			return nil
		}
	}
	// not reach
}

// Kill terminates the stage
func (s *Stage) Kill() error {
	if !s.Running {
		return nil
	}

	if s.Cancel != nil {
		s.Cancel()
		s.Running = false
		s.RunnerDict = make(map[string]*TaskRunner)
	}
	return nil
}
