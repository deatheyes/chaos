package server

import (
	"context"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/deatheyes/chaos/common"
	"github.com/deatheyes/chaos/nemesis"
	"github.com/deatheyes/chaos/pb"
	"github.com/golang/glog"
	"google.golang.org/grpc"
)

// Agent deployed with the nodes of cluster
type Agent struct {
	AddressGRPC       string            // GRPC listen address
	ControllerAddress string            // Controller GRPC address
	Role              string            // node role
	Selector          map[string]string // selector
	SelectorStr       string            // selector string
	Version           string            // version
	NemesisManager    *nemesis.Manager  // nemesis manager
}

// NewAgent create a Agent
func NewAgent(config map[string]string) *Agent {
	return &Agent{
		AddressGRPC:       config[FIGRPC],
		ControllerAddress: config[FIControllerGRPC],
		Role:              config[FINodeRole],
		SelectorStr:       config[FISelector],
		Selector:          make(map[string]string),
		Version:           "0.1",
		NemesisManager:    nemesis.NewManager(),
	}
}

func (a *Agent) init() {
	if len(a.AddressGRPC) == 0 {
		a.AddressGRPC = os.Getenv(FIGRPC)
	}

	if len(a.ControllerAddress) == 0 {
		a.ControllerAddress = os.Getenv(FIControllerGRPC)
	}

	if len(a.Role) == 0 {
		a.Role = os.Getenv(FINodeRole)
	}

	if len(a.SelectorStr) == 0 {
		a.SelectorStr = os.Getenv(FISelector)
	}

	// parse selector
	segments := strings.Split(a.SelectorStr, ",")
	for _, v := range segments {
		seg := strings.Split(v, "=")
		if len(seg) != 2 {
			glog.Warningf("[Agent] unexcepted selector %s", v)
		}
		a.Selector[seg[0]] = seg[1]
	}
}

// Run serve as a agent
func (a *Agent) Run() error {
	a.init()
	glog.Infof("[Agent] agent init done: %v", a)
	//register to controller
	for {
		if err := a.Register(); err != nil {
			glog.Warningf("[Agent] register to controller failed")
		} else {
			glog.Info("[Agent] register to controller done")
			break
		}
		time.Sleep(3 * time.Second)
	}

	// run grpc
	listen, err := net.Listen("tcp", a.AddressGRPC)
	if err != nil {
		glog.Fatalf("[Agent] listen to %s failed: %v", a.AddressGRPC)
		return err
	}

	s := grpc.NewServer()
	pb.RegisterAgentServer(s, a)
	s.Serve(listen)
	return nil
}

// Register report node info to controller
func (a *Agent) Register() error {
	conn, err := grpc.Dial(a.ControllerAddress, grpc.WithInsecure())
	if err != nil {
		glog.Fatalf("[Agent] connect to controller %s failed: %v", a.ControllerAddress, err)
		return err
	}

	client := pb.NewControllerClient(conn)
	req := &pb.RegisterRequest{Role: a.Role, Selector: a.Selector, Version: a.Version, Address: a.AddressGRPC}
	_, err = client.Register(context.Background(), req)
	if err != nil {
		glog.Warningf("[Agent] register to controller %s failed: %v", a.ControllerAddress, err)
		return err
	}
	// TODO: process response
	return nil
}

// Exec run a command at the agent
func (a *Agent) Exec(ctx context.Context, req *pb.ExecRequest) (*pb.ExecResponse, error) {
	// only run command sync supported currently
	glog.Infof("[Agent] execute command %s", req.Command)
	cmd := exec.Command("bash", "-c", req.Command)
	if err := cmd.Run(); err != nil {
		glog.Warningf("[Agent] run command %s failed: %v", req.Command, err)
		return &pb.ExecResponse{Code: 0}, err
	}
	glog.Infof("[Agent] execute command %s done", req.Command)
	return &pb.ExecResponse{Code: 0}, nil
}

// Nemesis manage a nemesis at the agent
func (a *Agent) Nemesis(ctx context.Context, req *pb.NemesisRequest) (*pb.NemesisResponse, error) {
	task := &nemesis.Task{Name: req.GetName(), Type: int(req.GetType()), ID: req.GetID(), Arguments: req.GetArguments()}
	if req.GetOp() == common.OperationStart {
		task.Op = nemesis.TaskCreate
		id, err := a.NemesisManager.CreateNemesis(task)
		if err != nil {
			glog.Warningf("[Agent] create nemesis task %v failed: %v", task, err)
			return &pb.NemesisResponse{Code: 1}, err
		}
		return &pb.NemesisResponse{Code: 0, ID: id}, nil
	}

	task.Op = nemesis.TaskTerminate
	if err := a.NemesisManager.TerminateNemesis(task); err != nil {
		glog.Warningf("[Agent] terminate nemesis task %v failed: %v", task, err)
		return &pb.NemesisResponse{Code: 1}, err
	}
	return &pb.NemesisResponse{Code: 0, ID: task.ID}, nil
}
