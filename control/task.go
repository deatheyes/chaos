package control

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/deatheyes/chaos/common"
	"github.com/deatheyes/chaos/config"
	"github.com/deatheyes/chaos/pb"
	"github.com/deatheyes/chaos/util"
	"github.com/golang/glog"
	"google.golang.org/grpc"
)

// Task or Stage status
const (
	StatusWaiting = iota
	StatusReady
	StatusRunning
	StatusDone
	StatusTerminating
	StautsTerminated
	StatusError
)

// Task interface
type Task interface {
	GetMeta() *TaskMeta
	Terminate(context.Context) error
	Execute(context.Context) error
	GetLastTarget() string
}

// TaskMeta contains the base info of a task
type TaskMeta struct {
	ID         string          // task unique id
	Type       int             // task type
	Group      string          // task group name
	Name       string          // task name
	Upstream   map[string]Task // requires
	Downstream map[string]Task // privodes
}

// key of arguments in nemesis
const (
	Ticker = "ticker"
	Timer  = "timer"
	Delay  = "delay"
)

// EmptyTask is used as the header of stage and debug
type EmptyTask struct {
	Meta *TaskMeta
}

// NewEmptyTask create a new empty task
func NewEmptyTask(name string) *EmptyTask {
	t := &EmptyTask{}
	meta := &TaskMeta{
		ID:         util.NewUUID(),
		Type:       common.TaskEmpty,
		Name:       name,
		Upstream:   make(map[string]Task),
		Downstream: make(map[string]Task),
	}
	t.Meta = meta
	return t
}

// GetMeta implement Task
func (t *EmptyTask) GetMeta() *TaskMeta {
	return t.Meta
}

// Terminate implement Task
func (t *EmptyTask) Terminate(context.Context) error {
	glog.V(8).Infof("[Task] task %s %s terminated", t.Meta.Name, t.Meta.ID)
	return nil
}

// Execute implement Task
func (t *EmptyTask) Execute(context.Context) error {
	glog.V(8).Infof("[Task] task %s %s executed", t.Meta.Name, t.Meta.ID)
	return nil
}

// GetLastTarget implement Task
func (t *EmptyTask) GetLastTarget() string {
	return ""
}

// Node contains necessary info of a node
type Node struct {
	Agent     *config.Agent // agent to process
	NemesisID string        // unique id of a instance of nemesis
}

// GetAddress return the address
func (n *Node) GetAddress() string {
	return n.Agent.Address
}

// ServiceTask control a instance of nemesis at the service level
type ServiceTask struct {
	Meta           *TaskMeta              // task meta
	TargetList     []*Node                // targets to process
	Arguments      map[string]string      // arguments to run nemesis
	SelectFunction func() []*config.Agent // function to select targets
	LastTargetInfo string                 // return by GetLastTarget
	sync.Mutex
}

// NewServiceTask create a new task to control a nemesis instance
func NewServiceTask(selectFunction func() []*config.Agent, name string, arguments map[string]string, typ int) *ServiceTask {
	t := &ServiceTask{
		Arguments:      arguments,
		SelectFunction: selectFunction,
	}
	meta := &TaskMeta{
		ID:         util.NewUUID(),
		Type:       typ,
		Group:      "Nemesis|Service",
		Name:       name,
		Upstream:   make(map[string]Task),
		Downstream: make(map[string]Task),
	}
	t.Meta = meta
	return t
}

func (t *ServiceTask) String() string {
	return fmt.Sprintf("%s-%s-%s", t.Meta.Group, t.Meta.ID, common.TaskType(t.Meta.Type))
}

// execute send the task to the agent
func (t *ServiceTask) execute(ctx context.Context, operation int) error {
	segment := []string{}
	for _, target := range t.TargetList {
		glog.V(8).Infof("[Task][%s] dispatch to %s", t.String(), target.GetAddress())
		conn, err := grpc.Dial(target.GetAddress(), grpc.WithInsecure())
		if err != nil {
			glog.Warningf("[Task][%s] create connection to %s failed: %v", t.String(), target.GetAddress(), err)
			return err
		}
		defer conn.Close()

		c := pb.NewAgentClient(conn)
		req := &pb.NemesisRequest{
			Type:      int32(t.Meta.Type),
			Name:      t.Meta.Name,
			Arguments: t.Arguments,
			ID:        target.NemesisID,
			Op:        int32(operation),
		}

		r, err := c.Nemesis(ctx, req)
		if err != nil {
			glog.Warningf("[Task][%s] control nemesis at %s with operation %d failed: %v", t.String(), target.GetAddress(), operation, err)
			return err
		}
		if r.GetCode() != 0 {
			glog.Warningf("[Task][%s] contol nemesis at %v with operation %d error: %s", t.String(), target.GetAddress(), operation, r.GetErrorMessage())
			return errors.New("nemesis operation failed")
		}
		// record the nemesis instance id for clean up
		target.NemesisID = r.GetID()
		glog.V(8).Infof("[Task][%s] process nemesis %s done", t.String(), target.NemesisID)
		segment = append(segment, target.GetAddress())
	}
	t.LastTargetInfo = strings.Join(segment, ",")
	return nil
}

// GetMeta implement Task
func (t *ServiceTask) GetMeta() *TaskMeta {
	return t.Meta
}

// Execute implement Task
func (t *ServiceTask) Execute(ctx context.Context) error {
	glog.V(8).Infof("[Task][%s] execute", t.String())
	if len(t.TargetList) != 0 {
		// status must be reset before execution
		glog.Warningf("[Task][%s] detect current execution which is not supported currently", t.String())
	}

	for _, target := range t.SelectFunction() {
		t.TargetList = append(t.TargetList, &Node{Agent: target})
	}
	if len(t.TargetList) == 0 {
		glog.Warningf("[Task][%s] no target to exeucte", t.String())
		return errors.New("no target")
	}
	// TODO: partitial failed and successed
	return t.execute(ctx, common.OperationStart)
}

// Terminate implement Task
func (t *ServiceTask) Terminate(ctx context.Context) error {
	glog.V(8).Infof("[Task][%s] terminate", t.String())
	if len(t.TargetList) == 0 {
		glog.Warningf("[Task][%s] no target to exeucte", t.String())
		return errors.New("no target")
	}
	defer func() {
		// clean status
		t.TargetList = []*Node{}
	}()

	// TODO: partital failed and successed
	return t.execute(ctx, common.OperationStop)
}

// GetLastTarget implement Task
func (t *ServiceTask) GetLastTarget() string {
	return t.LastTargetInfo
}

// NewIOTask create a new task to control a nemesis instance
func NewIOTask(selectFunction func() []*config.Agent, name string, arguments map[string]string, typ int) *ServiceTask {
	t := &ServiceTask{
		Arguments:      arguments,
		SelectFunction: selectFunction,
	}
	meta := &TaskMeta{
		ID:         util.NewUUID(),
		Type:       typ,
		Group:      "Nemesis|IO",
		Name:       name,
		Upstream:   make(map[string]Task),
		Downstream: make(map[string]Task),
	}
	t.Meta = meta
	return t
}

// TickerTask is a wrapper which setup a statefull Task with ticker
type TickerTask struct {
	T      Task          // inner task to run
	Delay  time.Duration // duration for a task to run
	Last   time.Duration // duration for a task to last
	Cycle  time.Duration // cycle to trigger the task
	Cancle func()        // cancle function
}

// NewTickerTask create a TickerTask
func NewTickerTask(t Task, last, cycle time.Duration) *TickerTask {
	return &TickerTask{
		T:     t,
		Delay: common.TickerDefaultDelay,
		Last:  last,
		Cycle: cycle,
	}
}

// GetMeta implement Task
func (t *TickerTask) GetMeta() *TaskMeta {
	return t.T.GetMeta()
}

// Execute implement Task
func (t *TickerTask) Execute(ctx context.Context) error {
	timer := time.NewTimer(t.Delay)
	count := 0
	c, cancle := context.WithCancel(ctx)
	t.Cancle = cancle

	for {
		select {
		case <-timer.C:
			count = (count + 1) % 2
			if count == 1 {
				glog.V(8).Info("[Task][Ticker] run sub task")
				go func() {
					if err := t.T.Execute(c); err != nil {
						glog.Warningf("[Task] execute task %s failed: %v", t.T.GetMeta().ID, err)
					}
				}()
				timer.Reset(t.Last)
			} else {
				glog.V(8).Info("[Task][Ticker] stop sub task")
				go func() {
					if err := t.T.Terminate(ctx); err != nil {
						glog.Warningf("[Task] terminate task %s failed: %v", t.T.GetMeta().ID, err)
					}
				}()
				timer.Reset(t.Cycle)
			}
		case <-ctx.Done():
			timer.Stop()
			glog.Infof("[Task] cancel task %s", t.GetMeta().ID)
			if err := t.T.Terminate(context.Background()); err != nil {
				glog.Warningf("[Task] terminate task %s failed: %v", t.GetMeta().ID, err)
			}
			return nil
		}
	}
}

// Terminate implement Task
func (t *TickerTask) Terminate(ctx context.Context) error {
	if t.Cancle != nil {
		t.Cancle()
	}
	return t.T.Terminate(ctx)
}

// GetLastTarget implement Task
func (t *TickerTask) GetLastTarget() string {
	return t.T.GetLastTarget()
}

// WithTicker wrap a task with ticker
func WithTicker(t Task, last, cycle time.Duration) Task {
	return NewTickerTask(t, last, cycle)
}

// TimerTask setup a long running task with timeout
type TimerTask struct {
	T       Task
	Timeout time.Duration
	Cancel  func()
}

// NewTimerTask setup a Task with timeout
func NewTimerTask(t Task, timeout time.Duration) *TimerTask {
	return &TimerTask{
		T:       t,
		Timeout: timeout,
	}
}

// GetMeta implement Task
func (t *TimerTask) GetMeta() *TaskMeta {
	return t.T.GetMeta()
}

// Execute implement Task
func (t *TimerTask) Execute(ctx context.Context) error {
	c, cancel := context.WithTimeout(ctx, t.Timeout)
	t.Cancel = cancel
	go func() {
		t.T.Execute(c)
	}()
	return nil
}

// Terminate implement Task
func (t *TimerTask) Terminate(ctx context.Context) error {
	if t.Cancel != nil {
		t.Cancel()
	}
	return t.T.Terminate(ctx)
}

// GetLastTarget implement Task
func (t *TimerTask) GetLastTarget() string {
	return t.T.GetLastTarget()
}

// Partition contains parts of the targets
type Partition []*Node

func (p Partition) String() string {
	list := []string{}
	for _, n := range p {
		list = append(list, n.GetAddress())
	}
	return strings.Join(list, ",")
}

// PartitionTask is special NemesisTask which create a partition with targets
type PartitionTask struct {
	Meta           *TaskMeta
	TargetList     []*config.Agent        // Agents to be selected
	Arguments      map[string]string      // arguments to run nemesis
	SelectFunction func() []*config.Agent // function to select targets
	Partitions     []Partition            // parts of targets
	LastTargetInfo string                 // return by GetLastTarget
}

// NewPartitionTask create a instance of partiton nemesis with specified targets
func NewPartitionTask(selectFunction func() []*config.Agent, name string, arguments map[string]string) *PartitionTask {
	t := &PartitionTask{
		Arguments:      arguments,
		SelectFunction: selectFunction,
		TargetList:     make([]*config.Agent, 6),
	}
	meta := &TaskMeta{
		ID:         util.NewUUID(),
		Type:       common.TaskPartition,
		Group:      "Nemesis|Partition",
		Name:       name,
		Upstream:   make(map[string]Task),
		Downstream: make(map[string]Task),
	}
	t.Meta = meta
	return t
}

func (t *PartitionTask) String() string {
	return fmt.Sprintf("%s-%s-%s", t.Meta.Group, t.Meta.ID, common.TaskType(t.Meta.Type))
}

// only two parts supported currently
func (t *PartitionTask) createParts() {
	if len(t.TargetList) < 2 {
		return
	}

	t.Partitions = []Partition{Partition{}, Partition{}}
	for _, target := range t.TargetList {
		id := rand.Int() % 2
		t.Partitions[id] = append(t.Partitions[id], &Node{Agent: target})
	}

	for i, part := range t.Partitions {
		if len(part) == 0 {
			peer := t.Partitions[(i+1)%2]
			id := rand.Int() % len(peer)
			part = append(part, peer[id])
			peer = append(peer[:id], peer[id+1:]...)
			break
		}
	}
}

func (t *PartitionTask) execute(ctx context.Context, part1, part2 Partition) error {
	// fill arguments
	arguments := make(map[string]string)
	for k, v := range t.Arguments {
		arguments[k] = v
	}
	nodes := []string{}
	for _, v := range part2 {
		nodes = append(nodes, v.Agent.Address)
	}
	arguments[common.ArgumentsKeyNodes] = strings.Join(nodes, common.ArgumentsSeparator)

	// execute
	for _, target := range part1 {
		conn, err := grpc.Dial(target.GetAddress(), grpc.WithInsecure())
		if err != nil {
			glog.Warningf("[Task][%s] create connection to %s failed: %v", t.String(), target.GetAddress(), err)
			return err
		}
		defer conn.Close()

		c := pb.NewAgentClient(conn)
		req := &pb.NemesisRequest{
			Type:      int32(t.Meta.Type),
			Name:      t.Meta.Name,
			Arguments: arguments,
			Op:        common.OperationStart,
			ID:        target.NemesisID,
		}
		r, err := c.Nemesis(ctx, req)
		if err != nil {
			glog.Warningf("[Task][%s] control nemesis at %s with operation %d failed: %v", t.String(), target.GetAddress(), common.OperationStart, err)
			return err
		}
		if r.GetCode() != 0 {
			glog.Warningf("[Task][%s] contol nemesis at %s with operation %d error: %s", t.String(), target.GetAddress(), common.OperationStart, r.GetErrorMessage())
			return errors.New("nemesis operation failed")
		}
		// record the nemesis instance id for clean up
		target.NemesisID = r.GetID()
	}
	t.LastTargetInfo = part1.String() + "|" + part2.String()
	return nil
}

// GetMeta implement Task
func (t *PartitionTask) GetMeta() *TaskMeta {
	return t.Meta
}

// Execute implement Task
func (t *PartitionTask) Execute(ctx context.Context) error {
	t.TargetList = t.SelectFunction()
	if len(t.TargetList) < 2 {
		glog.Infof("[Task][%s] only %d targets, no need to process", t.String(), len(t.TargetList))
		return nil
	}

	t.createParts()
	if err := t.execute(ctx, t.Partitions[0], t.Partitions[1]); err != nil {
		glog.Warningf("[Task][%s] create partition [%s][%s] failed: %v", t.String(), t.Partitions[0].String(), t.Partitions[1].String(), err)
		return err
	}

	if err := t.execute(ctx, t.Partitions[1], t.Partitions[0]); err != nil {
		glog.Warningf("[Task][%s] create partition [%s][%s] failed: %v", t.String(), t.Partitions[1].String(), t.Partitions[0].String(), err)
		return err
	}
	glog.Infof("[Task][%s] create partition [%s][%s] successed", t.String(), t.Partitions[0].String(), t.Partitions[1].String())
	return nil
}

// Terminate implement Task
func (t *PartitionTask) Terminate(ctx context.Context) error {
	defer func() {
		// clean
		// NOTE: would cause unreset status
		// TODO: fix it
		t.Partitions = []Partition{}
	}()

	for _, p := range t.Partitions {
		for _, target := range p {
			if len(target.NemesisID) == 0 {
				// it seems the nemesis not executed in Execution
				continue
			}

			conn, err := grpc.Dial(target.Agent.Address, grpc.WithInsecure())
			if err != nil {
				glog.Warningf("[Task][%s] create connection to %s failed: %v", t.String(), target.GetAddress(), err)
				return err
			}
			defer conn.Close()

			c := pb.NewAgentClient(conn)
			req := &pb.NemesisRequest{
				Type:      int32(t.Meta.Type),
				Name:      t.Meta.Name,
				Arguments: t.Arguments,
				Op:        common.OperationStop,
				ID:        target.NemesisID,
			}
			r, err := c.Nemesis(ctx, req)
			if err != nil {
				glog.Warningf("[Task][%s] control nemesis at %s with operation %d failed: %v", t.String(), target.GetAddress(), common.OperationStop, err)
				return err
			}
			if r.GetCode() != 0 {
				glog.Warningf("[Task][%s] contol nemesis at %s with operation %d error: %s", t.String(), target.GetAddress(), common.OperationStop, r.GetErrorMessage())
				return errors.New("nemesis operation failed")
			}
		}
	}

	if len(t.Partitions) > 0 {
		glog.Infof("[Task][%s] terminate partition [%s][%s] successed", t.String(), t.Partitions[0].String(), t.Partitions[1].String())
	}
	return nil
}

// GetLastTarget implement Task
func (t *PartitionTask) GetLastTarget() string {
	return t.LastTargetInfo
}

// Dispatcher is responseble to transport task to agent with rpc
type Dispatcher interface {
	Dispatch(t *Task) error
}

// TaskManager build stage with tasks, currently only static stages supported
// TODO dynamic stages
type TaskManager struct {
	Name            string                   // name for loggging
	Topology        config.Topology          // cluster topology
	RunnerDict      map[string]*TaskRunner   // dictionary to register runner
	Stop            chan struct{}            // stop notifier
	CancelFunction  func()                   // cancel all the tasks
	DataDir         string                   // dir to load stage
	StageDict       map[string]*Stage        // stage map
	StageConfigDict map[string]*config.Stage // stage configuration map
	sync.Mutex
}

// NewTaskManager create a task manager
func NewTaskManager() *TaskManager {
	return &TaskManager{
		Name:            "TaskManager",
		RunnerDict:      make(map[string]*TaskRunner),
		Topology:        make(config.Topology, 0),
		Stop:            make(chan struct{}),
		DataDir:         "./data",
		StageDict:       make(map[string]*Stage),
		StageConfigDict: make(map[string]*config.Stage),
	}
}

// Init setup task manager
func (m *TaskManager) Init() error {
	list, err := config.LoadStageFromDir(m.DataDir)
	if err != nil {
		return err
	}
	glog.V(8).Infof("[%s] init with stages %v", m.Name, list)
	// load preset stages configuration
	for _, c := range list {
		_, ok := m.StageConfigDict[c.Name]
		if ok {
			err := fmt.Errorf("duplicated stage %s", c.Name)
			glog.Warningf("[%s] %s", m.Name, err.Error())
			return err
		}
		m.StageConfigDict[c.Name] = c
	}
	return nil
}

// Register try adding a agent into topology
func (m *TaskManager) Register(a *config.Agent) {
	if _, ok := m.Topology[a.Address]; !ok {
		m.Topology[a.Address] = a
	} else {
		glog.Infof("[%s] agent ping: %v", m.Name, a)
	}
	glog.Infof("[%s] register agent: %v", m.Name, a)
}

// key of head task
const (
	Head      = "head"
	PARTITION = "PARTITION"
	SERVICE   = "SERVICE"
	IOERROR   = "IOERROR"
	IODELAY   = "IODELAY"
	EMPTY     = "EMPTY"
)

// BuildStage create a stage according to config, only task DAG current
// TODO: Stage DAG
func (m *TaskManager) BuildStage(c *config.Stage) (*Stage, error) {
	glog.V(8).Infof("[%s] build stage %s", m.Name, config.StageToString(c))

	s := NewStage(c.Name)
	s.TaskDict[Head] = NewEmptyTask(Head)
	queue := []string{Head}
	for {
		if len(queue) == 0 {
			break
		}

		v := queue[0]
		queue = queue[1:]

		glog.Infof("[%s] %s %v", m.Name, v, c.TaskRequireDict)
		if _, ok := c.TaskRequireDict[v]; !ok {
			continue
		}

		for _, t := range c.TaskRequireDict[v] {
			// build relations
			glog.Infof("[%s] %s %s", m.Name, v, t.Name)
			var task Task
			if len(t.ID) == 0 {
				task = m.BuildTask(t)
				s.TaskDict[t.Name] = task
				queue = append(queue, t.Name)
			} else {
				task = s.TaskDict[t.ID]
			}
			s.TaskDict[v].GetMeta().Downstream[task.GetMeta().ID] = task
			task.GetMeta().Upstream[v] = s.TaskDict[v]
		}
	}
	return s, nil
}

// BuildTask create a task and select a agent for the task
func (m *TaskManager) BuildTask(c *config.Task) Task {
	var t Task
	selectFunction := func() []*config.Agent {
		list := m.Topology.Select(c.Strategy.ToSelector())
		glog.V(8).Infof("[Manager] %d nodes availiable, limit: %d", len(list), c.Strategy.Limit)
		if c.Strategy.Limit > 0 && len(list) > c.Strategy.Limit {
			for len(list) > c.Strategy.Limit {
				id := rand.Int() % len(list)
				list = append(list[:id], list[id+1:]...)

			}
		}
		glog.V(8).Infof("[Manager] %d nodes selected", len(list))
		return list
	}
	switch strings.ToUpper(c.Type) {
	case PARTITION:
		p := NewPartitionTask(selectFunction, c.Name, c.Arguments)
		glog.V(8).Infof("[Manager] create partition task [%s]", p.String())
		t = p
	case SERVICE:
		s := NewServiceTask(selectFunction, c.Name, c.Arguments, common.TaskService)
		glog.V(8).Infof("[Manager] create service task [%s]", s.String())
		t = s
	case IOERROR:
		s := NewIOTask(selectFunction, c.Name, c.Arguments, common.TaskIOError)
		glog.V(8).Infof("[Manager] create io error task [%s],", s.String())
		t = s
	case IODELAY:
		s := NewIOTask(selectFunction, c.Name, c.Arguments, common.TaskIODelay)
		glog.V(8).Infof("[Manager] create io delay task [%s],", s.String())
		t = s
	case EMPTY:
		t = NewEmptyTask(c.Name)
	default:
		glog.Warningf("[%s] unkonwn task type: %s", m.Name, c.Type)
		t = NewEmptyTask(c.Name)
	}

	// setup wrapper
	if c.Strategy.TickerCycle > 0 && c.Strategy.TickerLast > 0 {
		last := time.Duration(c.Strategy.TickerLast) * time.Second
		cycle := time.Duration(c.Strategy.TickerCycle) * time.Second
		t = NewTickerTask(t, last, cycle)
		glog.Infof("[%s][%s][%s][%s] setup with ticker, last: %v cycle: %v", m.Name, t.GetMeta().Group, t.GetMeta().Name, t.GetMeta().ID, last, cycle)
	}

	if c.Strategy.Timeout > 0 {
		timeout := time.Duration(c.Strategy.Timeout) * time.Second
		t = NewTimerTask(t, timeout)
		glog.Infof("[%s][%s][%s][%s] setup with timer, timeout: %v", m.Name, t.GetMeta().Group, t.GetMeta().Name, t.GetMeta().ID, timeout)
	}
	return t
}

// Errors
var (
	ErrNotFound       = errors.New("NotFound")
	ErrAlreadyRunning = errors.New("AlreadyRunning")
	ErrDuplicated     = errors.New("Duplicated")
)

// RunStage start a stage by name
func (m *TaskManager) RunStage(name string) error {
	m.Lock()
	defer m.Unlock()

	if _, ok := m.StageConfigDict[name]; !ok {
		return ErrNotFound
	}

	if old, ok := m.StageDict[name]; !ok || !old.Running {
		// build and run
		s, err := m.BuildStage(m.StageConfigDict[name])
		if err != nil {
			glog.Warningf("[%s] build stage %s failed: %v", m.Name, name, err)
			return err
		}
		glog.Infof("[%s] build stage %s done", m.Name, name)
		m.StageDict[name] = s
		s.Running = true
		ctx, cancel := context.WithCancel(context.Background())
		s.Cancel = cancel
		go s.Run(ctx)
		return nil
	}
	glog.Warningf("[%s] stage %s is running", m.Name, name)
	return ErrAlreadyRunning
}

// KillStage terminate stage by name
func (m *TaskManager) KillStage(name string) error {
	m.Lock()
	defer m.Lock()

	s, ok := m.StageDict[name]
	if !ok {
		return ErrNotFound
	}
	if !s.Running {
		return nil
	}
	return m.StageDict[name].Kill()
}
