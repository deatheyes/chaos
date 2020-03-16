package nemesis

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	"github.com/deatheyes/chaos/common"
	"github.com/deatheyes/chaos/util"
	"github.com/golang/glog"
)

// nemesis status
const (
	StatusOK = iota
	StatusError
)

// Nemesis interface
type Nemesis interface {
	GetID() string
	GetType() int
	GetStatus() int
	Run() error
	Terminate() error
}

// Partition disable the communication with specified nodes
// Note: for the sake of safty, all the nodes of partition must not be the controller
// TODO: unilateral disable
type Partition struct {
	ID           string   // unique id
	Type         int      // nemesis type
	Status       int      // runtime status
	LasterError  error    // last error
	RejectedNode []string // nodes to disable
	Debug        bool     // if is in debug mod
}

// NewPartition create a new partition nemesis
func NewPartition(agents []string) *Partition {
	return &Partition{
		ID:           util.NewUUID(),
		Type:         common.TaskPartition,
		Status:       StatusOK,
		RejectedNode: agents,
	}
}

// GetID implement Nemesis
func (p *Partition) GetID() string {
	return p.ID
}

// GetType implement Nemesis
func (p *Partition) GetType() int {
	return p.Type
}

// GetStatus implement Nemesis
func (p *Partition) GetStatus() int {
	return p.Status
}

// Run start the nemesis with inner run function
func (p *Partition) Run() error {
	if p.Debug {
		glog.Infof("[Nemesis][Partition][Run][Debug] id: %s, reject nodes: %s", p.ID, p.RejectedNode)
		return nil
	}

	for _, v := range p.RejectedNode {
		segments := strings.Split(strings.Trim(v, " "), ":")
		if len(segments) != 2 {
			glog.Warningf("[Nemesis] type %v id %s invalidate address %s", p.Type, p.ID, v)
			continue
		}

		ip := segments[0]
		// TODO make use of port for precise
		//port := segments[1]
		command := fmt.Sprintf("iptables -A INPUT -s %s -j DROP -w", ip)
		out, err := util.RunCommand(command)
		if err != nil {
			glog.Warningf("[Nemesis] type %v ID %s run command %s failed: %v, output: %s", p.Type, p.ID, command, err, string(out))
			p.LasterError = err
			p.Status = StatusError
			return err
		}
		glog.V(8).Infof("[Nemesis] type %v ID %s run commad %s done: %s", p.Type, p.ID, command, string(out))
	}
	return nil
}

// Terminate ends the nemesis with inner terminate function
func (p *Partition) Terminate() error {
	if p.Debug {
		glog.Infof("[Nemesis][Partition][Terminate][Debug] id: %s", p.ID)
		return nil
	}
	// TODO precisely clean
	// do not care about the output and error currently
	command := "iptables -F -w; iptables -X -w"
	_, err := util.RunCommand(command)
	if err != nil {
		return err
	}
	glog.V(8).Infof("[Nemesis] type %v ID %s run command %s done", p.Type, p.ID, command)
	return nil
}

// Service process service level nemesis
type Service struct {
	ID            string // unique id
	Type          int    // nemesis type
	Status        int    // runtime status
	LasterError   error  // last error
	CommandRun    string // command to kill the process
	CommandRevive string // command to revive the process
	Debug         bool   // if is in debug mod
}

// NewService create a nemesis to stop and revive a process
func NewService(run, revive string) *Service {
	return &Service{
		ID:            util.NewUUID(),
		Type:          common.TaskService,
		Status:        StatusOK,
		CommandRun:    run,
		CommandRevive: revive,
		Debug:         false,
	}
}

// GetID implement Nemesis
func (s *Service) GetID() string {
	return s.ID
}

// GetType implement Nemesis
func (s *Service) GetType() int {
	return s.Type
}

// GetStatus implement Nemesis
func (s *Service) GetStatus() int {
	return s.Status
}

// Run start the nemesis
func (s *Service) Run() error {
	if s.Debug {
		glog.Infof("[Nemesis][Service][Run][Debug] id: %s, command: %s", s.ID, s.CommandRun)
		return nil
	}

	err := util.StartCommand(s.CommandRun)
	if err != nil {
		glog.Warningf("[Nemesis][Service][Run] exec command %s failed: %v", s.CommandRun, err)
		return err
	}
	glog.Infof("[Nemesis][Service] exec command %s done", s.CommandRun)
	return nil
}

// Terminate revives the process with CommandRevive
func (s *Service) Terminate() error {
	if s.Debug {
		glog.Infof("[Nemesis][Service][Run][Debug] id: %s, command: %s", s.ID, s.CommandRevive)
		return nil
	}

	err := util.StartCommand(s.CommandRevive)
	if err != nil {
		glog.Warningf("[Nemesis][Service][Terminate] exec command %s failed: %v", s.CommandRevive, err)
		return err
	}
	glog.Infof("[Nemesis][Service][Terminate] exec command %s done", s.CommandRevive)
	return nil
}

// Persistence manages the long running nemesis
type Persistence struct {
	ID          string    // unique id
	Type        int       // nemesis type
	Status      int       // runtime status
	LasterError error     // last error
	Command     string    // long running command
	Process     *exec.Cmd // inner command struct
	Debug       bool      // if is in debug mod
}

// NewPersistence create a new persistent nemesis
func NewPersistence(command string) *Persistence {
	return &Persistence{
		ID:      util.NewUUID(),
		Status:  StatusOK,
		Command: command,
	}
}

// Run start the persistent nemesis
func (p *Persistence) Run() error {
	glog.V(8).Infof("[Nemesis][Persistence][Run] command: %s", p.Command)
	// debug
	if p.Debug {
		glog.Infof("[Nemesis][Persistence][Run][Debug] command %s", p.Command)
		return nil
	}

	if p.Process != nil {
		return fmt.Errorf("command %s is running", p.Command)
	}

	p.Process = exec.Command("bash", "-c", p.Command)
	// set pgid to the same value as pid
	p.Process.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdErrPipe, err := p.Process.StderrPipe()
	if err != nil {
		glog.Warningf("[Nemesis][Persistence] command %s create std error pipe failed: %v", err)
	}
	stdOutPipe, err := p.Process.StdoutPipe()
	if err != nil {
		glog.Warningf("[Nemesis][Persistence] command %s create std out pipe failed: %v", err)
	}

	f := func(r io.Reader, key string) {
		reader := bufio.NewReader(r)
		for {
			line, err := reader.ReadString('\n')
			glog.Infof("[Nemesis][Persistence][%s][%s]: %s", p.Command, key, line)
			if err != nil {
				return
			}
		}
	}

	go f(stdErrPipe, "StdErr")
	go f(stdOutPipe, "StdOut")
	return p.Process.Start()
}

// Terminate stop the  persistent nemesis
func (p *Persistence) Terminate() error {
	defer func() {
		// clean
		p.Process = nil
	}()

	glog.V(8).Infof("[Nemesis][Persistence][Terminate] command: %s", p.Command)
	// debug
	if p.Debug {
		glog.Infof("[Nemesis][Persistence][Terminate][Debug] command %s", p.Command)
		return nil
	}

	if p.Process == nil {
		glog.Warningf("[Nemesis][Persistence] command %s is not running", p.Command)
		return nil
	}

	pgid, err := syscall.Getpgid(p.Process.Process.Pid)
	if err != nil {
		glog.Warningf("[Nemesis][Persistence] get pgid to kill command %s failed: %v", err)
	}
	defer func() {
		util.RunCommand("lsmod | grep stap | awk '{print $1}' | xargs -i -t rmmod {}")
	}()
	return syscall.Kill(-pgid, syscall.SIGKILL)
}

// Task operations
const (
	TaskCreate = iota
	TaskTerminate
)

// Task contains the infomation to operate a nemesis
type Task struct {
	Name      string
	Type      int
	ID        string
	Arguments map[string]string
	Op        int
}

// Manager create, kill and maintain the nemesis
type Manager struct {
	NemesisDict map[string]Nemesis // nemesis instances dict
	sync.Mutex
	//	In          chan *Task         // task input chan
	//	Stop        chan struct{}      // notifier to stop the loop
}

// NewManager create a new nemesis manager
func NewManager() *Manager {
	return &Manager{
		NemesisDict: make(map[string]Nemesis),
		//		In:          make(chan *Task),
	}
}

// errors to return
var (
	ErrNotFount       = errors.New("not founnd")
	ErrUnknown        = errors.New("unknown")
	ErrEmptyNodes     = errors.New("empty nodes")
	ErrInvalidateArgs = errors.New("invalidate arguments")
)

// CreateNemesis create or run a existed nemesis
func (m *Manager) CreateNemesis(task *Task) (string, error) {
	m.Lock()
	defer m.Unlock()

	// debug mod
	debug := false
	if v, ok := task.Arguments[common.ArgumentsDebug]; ok && v == "true" {
		debug = true
	}

	n, ok := m.NemesisDict[task.ID]
	if !ok || len(task.ID) == 0 {
		// create a new nemesis
		switch task.Type {
		case common.TaskPartition:
			nodes, ok := task.Arguments[common.ArgumentsKeyNodes]
			if !ok {
				glog.Warningf("[NemesisManager] no nodes to reject")
				return "", ErrEmptyNodes
			}
			agents := strings.Split(nodes, ",")
			nemesis := NewPartition(agents)
			nemesis.Debug = debug
			m.NemesisDict[nemesis.GetID()] = nemesis
			glog.Infof("[NemesisManager] create partition[%s] with nodes %s", nemesis.GetID(), nodes)
			return nemesis.GetID(), nemesis.Run()
		case common.TaskService:
			cmd, ok := task.Arguments[common.ArgumentsKeyRun]
			if !ok {
				glog.Warning("[NemesisManager] argument 'run' is expected to create service nemesis")
				return "", ErrInvalidateArgs
			}
			revive, ok := task.Arguments[common.ArgumentsKeyTerminate]
			if !ok {
				glog.Warning("[NemesisManager] argument 'terminate' is expected to create service nemesis")
				return "", ErrInvalidateArgs
			}

			nemesis := NewService(cmd, revive)
			nemesis.Debug = debug
			m.NemesisDict[nemesis.GetID()] = nemesis
			glog.Infof("[NemesisManager] create service[%s] with command %s and %s", nemesis.GetID(), cmd, revive)
			return nemesis.GetID(), nemesis.Run()
		case common.TaskIOError:
			nemesis, err := NewSTP(task.Arguments, SystemTapIOError)
			if err != nil {
				glog.Warningf("[NemesisManager] create systemtap IO error probe failed")
				return "", err
			}
			nemesis.P.Debug = debug
			m.NemesisDict[nemesis.GetID()] = nemesis
			glog.Infof("[NemesisManager] create systemtap io error probe[%s]", nemesis.GetID())
			return nemesis.GetID(), nemesis.Run()
		case common.TaskIODelay:
			nemesis, err := NewSTP(task.Arguments, SystemTapIODelay)
			if err != nil {
				glog.Warningf("[NemesisManager] create systemtap IO delay probe failed")
				return "", err
			}
			nemesis.P.Debug = debug
			m.NemesisDict[nemesis.GetID()] = nemesis
			glog.Infof("[NemesisManager] create systemtap io delay probe[%s]", nemesis.GetID())
			return nemesis.GetID(), nemesis.Run()
		default:
			glog.Warningf("[NemesisManager] unknown nemesis %s", task.Name)
			return "", ErrUnknown
		}
	}

	// existed nemesis
	return n.GetID(), n.Run()
}

// TerminateNemesis kill a existed nemesis
func (m *Manager) TerminateNemesis(task *Task) error {
	m.Lock()
	defer m.Unlock()

	glog.Infof("[NemesisManager] terminate nemesis[%s]", task.ID)
	n, ok := m.NemesisDict[task.ID]
	if !ok {
		return ErrNotFount
	}
	defer delete(m.NemesisDict, task.ID)
	return n.Terminate()
}

// TerminateAllNemesis kill all nemesis
func (m *Manager) TerminateAllNemesis() {
	glog.Info("[NemesisManager] terminate all nemesis")
	for _, v := range m.NemesisDict {
		v.Terminate()
	}
	m.NemesisDict = make(map[string]Nemesis)
}

// NumberOfNemesis returns total number of nemesis
func (m *Manager) NumberOfNemesis() int {
	m.Lock()
	defer m.Unlock()
	return len(m.NemesisDict)
}
