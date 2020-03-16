package control

import (
	"context"

	"github.com/golang/glog"
)

// TaskRunner is a wrapper to run task
type TaskRunner struct {
	In          chan Task // notify for upstream
	Out         chan Task // notify for downstram
	Count       int       // upstream counter
	T           Task      // task wrapped
	Name        string    // 'TaskRunner'
	LasterError error     // last error in execution
}

// NewTaskRunner create a new wapper to check and run task
func NewTaskRunner(t Task, out chan Task) *TaskRunner {
	return &TaskRunner{
		In:    make(chan Task),
		Out:   out,
		Count: 0,
		T:     t,
		Name:  "TaskRunner",
	}
}

// Run execute the task and proceed the step in DAG
func (r *TaskRunner) Run(ctx context.Context) {
	for {
		select {
		case require := <-r.In:
			// TODO: make use of upstream
			requireNum := len(r.T.GetMeta().Upstream)
			r.Count++
			if r.Count == requireNum {
				glog.V(8).Infof("[Runner][%s] execute task %s %s", r.Name, r.T.GetMeta().Group, r.T.GetMeta().ID)
				if err := r.T.Execute(ctx); err != nil {
					r.LasterError = err
					glog.Warningf("[Runner][%s] execute task %s %s failed: %s", r.Name, r.T.GetMeta().Group, r.T.GetMeta().ID, err)
				}
				go func() {
					r.Out <- r.T
				}()
			} else if r.Count > requireNum {
				glog.Errorf("[Runner][%s] DAG error detected at task %s %s count %d", r.Name, r.T.GetMeta().Group, r.T.GetMeta().ID, r.Count)
			} else {
				glog.V(8).Infof("[Runner][%s] task %s %s got require %s", r.Name, r.T.GetMeta().Group, r.T.GetMeta().ID, require.GetMeta().ID)
			}
		case <-ctx.Done():
			if err := r.T.Terminate(context.Background()); err != nil {
				r.LasterError = err
				glog.Warningf("[Runner][%s] task %s %s terminate failed", r.Name, r.T.GetMeta().Group, r.T.GetMeta().ID)
			} else {
				glog.V(8).Infof("[Runner][%s] task %s %s terminated", r.Name, r.T.GetMeta().Group, r.T.GetMeta().ID)
			}
			return
		}
	}
}
