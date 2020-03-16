package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/deatheyes/chaos/server"
	"github.com/golang/glog"
)

var (
	addressHTTP       string
	addressGRPC       string
	addressController string
	configPath        string
	role              string
	selector          string
	version           bool

	// build info
	sha1   string
	date   string
	branch string
)

func init() {
	flag.StringVar(&addressHTTP, "a", ":8677", "http address to listen")
	flag.StringVar(&addressGRPC, "g", ":8678", "grpc address to listen")
	flag.StringVar(&addressController, "l", "", "controller address, retrieved form env if not set, uesed by agent")
	flag.StringVar(&configPath, "c", "./conf/config.yaml", "config file, used only by controller currently")
	flag.StringVar(&selector, "s", "", "selector, retrieved from env if not set, used by agent. eg: app=test,flag=aaaaa")
	flag.StringVar(&role, "r", "agent", "run as [agent|controller]")
	flag.BoolVar(&version, "version", false, "show version")
}

func showVersion() {
	fmt.Printf("chaos - author yanyu - %s - %s - %s", branch, date, sha1)
}

func main() {
	flag.Parse()
	if version {
		showVersion()
		return
	}

	config := make(map[string]string)
	if role != server.FIValueAgent && role != server.FIValueController {
		glog.Fatalf("[Main] invalidate role %s", role)
		return
	}
	config[server.FIRole] = strings.Trim(role, " ")
	config[server.FIGRPC] = addressGRPC
	if role == server.FIValueAgent {
		config[server.FIControllerGRPC] = addressController
		config[server.FISelector] = selector
	} else {
		config[server.FIHTTP] = addressHTTP
	}
	config[server.ConfigPath] = configPath

	s := server.NewServer(config)
	if err := s.Run(); err != nil {
		glog.Fatalf("[Main] start server failed: %v", err)
	}
}
