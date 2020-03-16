package control

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/deatheyes/chaos/common"
	"github.com/deatheyes/chaos/config"
	"github.com/deatheyes/chaos/util"
)

type LongRunningTestTask struct {
	meta   *TaskMeta
	ticker *time.Ticker
	count  int
	ref    int
	stop   chan struct{}
	stoped bool
}

func newLongRunningTestTask() *LongRunningTestTask {
	meta := &TaskMeta{
		ID:         util.NewUUID(),
		Type:       common.TaskEmpty,
		Upstream:   make(map[string]Task),
		Downstream: make(map[string]Task),
		Name:       "LongRunningTestTask",
	}
	return &LongRunningTestTask{
		meta:   meta,
		stoped: true,
		stop:   make(chan struct{}),
	}
}

func (t *LongRunningTestTask) GetMeta() *TaskMeta {
	return t.meta
}

func (t *LongRunningTestTask) Execute(ctx context.Context) error {
	if !t.stoped {
		return nil
	}
	t.stoped = false
	t.stop = make(chan struct{})

	t.count++
	t.ref++
	t.ticker = time.NewTicker(1 * time.Second)
	for {
		select {
		case <-t.ticker.C:
			fmt.Printf("[%s|%s] ticker\n", t.meta.Name, t.meta.ID)
		case <-t.stop:
			fmt.Printf("[%s|%s] stop\n", t.meta.Name, t.meta.ID)
			return nil
		case <-ctx.Done():
			fmt.Printf("[%s|%s] done\n", t.meta.Name, t.meta.ID)
			return nil
		}
	}
}

func (t *LongRunningTestTask) Terminate(ctx context.Context) error {
	if t.stoped {
		return nil
	}

	close(t.stop)
	t.ref--
	if t.ticker != nil {
		t.ticker.Stop()
	}
	fmt.Printf("[%s|%s] teminate\n", t.meta.Name, t.meta.ID)
	t.stoped = true
	return nil
}

// GetLastTarget implement Task
func (t *LongRunningTestTask) GetLastTarget() string {
	return ""
}

func TestTimerTask(t *testing.T) {
	fmt.Println("TestTimerTask")
	inner := newLongRunningTestTask()
	wrapper := NewTimerTask(inner, 10*time.Second)
	start := time.Now()
	ctx := context.Background()
	go func() {
		wrapper.Execute(ctx)
	}()

	time.Sleep(5 * time.Second)
	wrapper.Terminate(context.Background())
	end := time.Now()
	if end.Sub(start) > 7*time.Second {
		t.Errorf("TimerTask faild, terminate long running task after 7 seconds")
	}
}

func TestTickerTask(t *testing.T) {
	fmt.Println("TestTickerTask")
	inner := newLongRunningTestTask()
	wrapper := NewTickerTask(inner, 3*time.Second, 5*time.Second)

	go func() {
		wrapper.Execute(context.Background())
	}()

	time.Sleep(20 * time.Second)
	wrapper.Terminate(context.Background())
	if inner.ref != 0 {
		t.Errorf("TickerTask error ref %d", inner.ref)
	}

	// 20 - 5(TickerDefualtDelay) / (3+5) +1
	if inner.count != 2 {
		t.Errorf("TickerTask run inner task %d expected %d", inner.count, 2)
	}
}

func TestTimerTickerTask(t *testing.T) {
	fmt.Println("TestTimerTickerTask")
	inner := newLongRunningTestTask()
	wrapper := NewTickerTask(inner, 3*time.Second, 5*time.Second)
	wrapper1 := NewTimerTask(wrapper, 20*time.Second)

	go func() {
		wrapper1.Execute(context.Background())
	}()

	time.Sleep(21 * time.Second)
	wrapper1.Terminate(context.Background())
}

func TestCreateParts(t *testing.T) {
	fmt.Println("TestCreateParts")
	task := NewPartitionTask(func() []*config.Agent {
		ret := []*config.Agent{}
		a1 := &config.Agent{Address: "127.0.0.1"}
		a2 := &config.Agent{Address: "127.0.0.2"}
		a3 := &config.Agent{Address: "127.0.0.3"}
		ret = append(ret, a1, a2, a3)
		return ret
	}, "t1", make(map[string]string))

	task.createParts()
	if len(task.Partitions) != 2 {
		t.Errorf("unexpect partition segments: %d", len(task.Partitions))
		return
	}

	if len(task.Partitions[0]) == 0 || len(task.Partitions[1]) == 0 {
		t.Errorf("empty partition segments found")
	}
}
