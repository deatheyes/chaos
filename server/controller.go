package server

import (
	"context"
	"errors"
	"net"
	"sort"
	"strings"

	"github.com/deatheyes/chaos/common"
	"github.com/deatheyes/chaos/config"
	"github.com/deatheyes/chaos/control"
	"github.com/deatheyes/chaos/pb"
	"github.com/golang/glog"
	iris "github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/middleware/logger"
	"github.com/kataras/iris/v12/middleware/recover"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"gopkg.in/yaml.v2"
)

// API constants
const (
	IDALL           = "all" // flag to get all stages' detail
	IDList          = "list"
	ConfigSeparator = "---\n" // separator to join configuration
	ListSeparator   = ","     // separator to join stage list
)

// APIReturn is the HTTP return value
type APIReturn struct {
	Status string                 `json:"status"` // process status
	Data   map[string]interface{} `json:"data"`   // result returned
}

// Controller is a impliment of GRPC
type Controller struct {
	AddressGRPC string
	AddressHTTP string
	ConfigPath  string
	TaskManager *control.TaskManager
	Config      *config.ControllerConfig
}

// NewController create a new Controller
func NewController(config map[string]string) *Controller {
	return &Controller{
		AddressGRPC: config[FIGRPC],
		AddressHTTP: config[FIHTTP],
		ConfigPath:  config[ConfigPath],
	}
}

// Run serve as a controller
func (c *Controller) Run() error {
	// load config
	conf, err := config.LoadControllerConfigFromFile(c.ConfigPath)
	if err != nil {
		glog.Warningf("[Controller] task manager load configuration faild: %v", err)
		return err
	}
	c.Config = conf
	// setup task manager
	c.TaskManager = control.NewTaskManager()
	if err := c.TaskManager.Init(); err != nil {
		glog.Fatalf("[Controller] task manager init failed:%v", err)
		return err
	}

	// run grpc
	listen, err := net.Listen("tcp", c.AddressGRPC)
	if err != nil {
		glog.Fatalf("[Controller] listen to %s failed: %v", c.AddressGRPC)
		return err
	}

	s := grpc.NewServer()
	pb.RegisterControllerServer(s, c)
	go func() {
		s.Serve(listen)
	}()

	// run RESET server
	app := iris.New()
	app.Logger().SetLevel("debug")
	app.Use(recover.New())
	app.Use(logger.New())
	app.Handle("GET", "/chaos/{id:string}", c.queryFaultInjection())
	app.Handle("POST", "/chaos/{id:string}", c.runFaultInjection())
	app.Handle("DELETE", "/chaos/{id:string}", c.removeFaultInjection())
	app.Handle("POST", "exec/raw", c.executeRawCommand())
	app.Run(iris.Addr(c.AddressHTTP))
	return nil
}

// Register is a impliment of pb.Controller
func (c *Controller) Register(ctx context.Context, r *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	// get the agent address
	pr, ok := peer.FromContext(ctx)
	if !ok {
		glog.Warning("[Controller] get agent info failed")
		return &pb.RegisterResponse{Code: 1}, errors.New("get agent info failed")
	}
	address := pr.Addr.String()
	segments := strings.Split(address, ":")
	if len(segments) != 2 {
		glog.Warningf("[Controller] invalidate agent address %s")
		return &pb.RegisterResponse{Code: 1}, errors.New("invalidate agent address")
	}
	glog.Infof("[Controller] agent %s ask for register", address)
	// register the agent
	a := &config.Agent{
		Role:     r.GetRole(),
		Address:  segments[0] + r.GetAddress(),
		Version:  r.GetVersion(),
		Selector: r.GetSelector(),
	}
	c.TaskManager.Register(a)
	return &pb.RegisterResponse{Code: 0}, nil
}

func (c *Controller) queryFaultInjection() func(ctx iris.Context) {
	return func(ctx iris.Context) {
		var ret string
		id := ctx.Params().Get("id")
		if id == IDList {
			// list all availiable stage
			list := []string{}
			for _, v := range c.TaskManager.StageDict {
				list = append(list, v.Name)
			}
			sort.Strings(list)
			ret = strings.Join(list, ListSeparator)
		} else if id == IDALL {
			// return all stages
			segments := []string{}
			for k, v := range c.TaskManager.StageConfigDict {
				data, err := yaml.Marshal(v)
				if err != nil {
					glog.Warningf("[Controller] marshal stage %s configuration failed: %v", k, err)
				} else {
					segments = append(segments, string(data))
				}
			}
			ret = strings.Join(segments, ConfigSeparator)
		} else {
			if v, ok := c.TaskManager.StageConfigDict[id]; !ok {
				ret = control.ErrNotFound.Error()
			} else {
				data, err := yaml.Marshal(v)
				if err != nil {
					glog.Warningf("[Controller] marshal stage %s configuration failed: %v", id, err)
					ret = err.Error()
				} else {
					ret = string(data)
				}
			}
		}
		ctx.WriteString(ret)
	}
}

func (c *Controller) runFaultInjection() func(ctx iris.Context) {
	return func(ctx iris.Context) {
		id := ctx.Params().Get("id")
		if err := c.TaskManager.RunStage(id); err != nil {
			ctx.WriteString(err.Error())
		}
	}
}

func (c *Controller) removeFaultInjection() func(ctx iris.Context) {
	return func(ctx iris.Context) {
		id := ctx.Params().Get("id")
		if _, ok := c.TaskManager.StageDict[id]; ok {
			c.TaskManager.StageDict[id].Kill()
			// temporary
			// return the first target if exists
			var out string
			for _, t := range c.TaskManager.StageDict[id].TaskDict {
				if len(t.GetLastTarget()) != 0 {
					out = t.GetLastTarget()
					break
				}
			}
			ctx.WriteString(out)
		} else {
			ctx.WriteString(control.ErrNotFound.Error())
		}
	}
}

// errors for execution of raw command
var (
	ErrNoCommand        = errors.New("NoCommand")
	ErrNoTarget         = errors.New("NoTarget")
	ErrUnknownTarget    = errors.New("UnknownTarget")
	ErrCreateConnection = errors.New("ConnectFailed")
)

func (c *Controller) executeRawCommand() func(ctx iris.Context) {
	return func(ctx iris.Context) {
		in := make(map[string]string)
		if err := ctx.ReadJSON(&in); err != nil {
			glog.Warningf("[Controller] read josn failed: %v", err)
			ctx.WriteString(err.Error())
			return
		}
		command, ok := in[common.ArgumentsKeyCommand]
		if !ok {
			glog.Warningf(ErrNoCommand.Error())
			ctx.WriteString(ErrNoCommand.Error())
			return
		}
		target, ok := in[common.ArgumentsKeyTarget]
		if !ok {
			glog.Warningf(ErrNoTarget.Error())
			ctx.WriteString(ErrNoTarget.Error())
			return
		}

		glog.Infof("[Controller] try to dispatch command %s to %s", command, target)
		c.TaskManager.Lock()
		defer c.TaskManager.Unlock()
		for k := range c.TaskManager.Topology {
			if k == target {
				// execute command
				conn, err := grpc.Dial(k, grpc.WithInsecure())
				if err != nil {
					glog.Warningf("[Controller] create connection to %s failed: %v", k, err)
					ctx.WriteString(ErrCreateConnection.Error())
					return
				}
				client := pb.NewAgentClient(conn)
				_, err = client.Exec(context.Background(), &pb.ExecRequest{Command: command, Background: false})
				if err != nil {
					glog.Warningf("[Controller] execute command %s at %s failed: %v", command, k, err)
					ctx.WriteString(err.Error())
					return
				}
				glog.Infof("[Controller] exexute command %s done", command)
				return
			}
		}
		ctx.WriteString(ErrUnknownTarget.Error())
	}
}
