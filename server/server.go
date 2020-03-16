package server

import (
	"github.com/golang/glog"
)

// Server intreface
type Server interface {
	Run() error
}

// configuration
var (
	FIValueAgent      = "agent"
	FIValueController = "controller"
	FIRole            = "FI_ROLE"
	FIHTTP            = "FI_HTTP"
	FIGRPC            = "FI_GRPC"
	FIControllerGRPC  = "FI_CONTROLLER_GRPC"
	FINodeRole        = "FI_NODE_ROLE"
	FISelector        = "FI_SELECTOR"

	ConfigPath = "path"
)

// NewServer create a new server by role
func NewServer(config map[string]string) Server {
	glog.Infof("[Server] created as %s", config[FIRole])
	if config[FIRole] != FIValueAgent {
		return NewController(config)
	}
	return NewAgent(config)
}
