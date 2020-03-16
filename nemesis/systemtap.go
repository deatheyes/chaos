package nemesis

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/deatheyes/chaos/common"
	"github.com/deatheyes/chaos/util"
	"github.com/golang/glog"
)

// write error
const (
	WEPERM  = 1
	WEINTR  = 4
	WEIO    = 5
	WEBADF  = 9
	WEINVAL = 22
	WEFBIG  = 27
	WENOSPC = 28
	WEDQUOT = 122
)

// read error
const (
	REINTR  = 4
	RIO     = 5
	REBADF  = 9
	REINVAL = 22
)

// error list
var (
	ReadErrorList  = []int{REINTR, RIO, REBADF, REINVAL}
	WriteErrorList = []int{WEPERM, WEINTR, WEIO, WEBADF, WEINVAL, WEFBIG, WENOSPC, WEDQUOT}
)

// SystemTapClass specifies which kind of script to run
type SystemTapClass int

// systemtap types
const (
	SystemTapIOError SystemTapClass = iota
	SystemTapIODelay
)

// SystemTap wrapper
type SystemTap struct {
	P         *Persistence   // persistent nemesis
	Arguments []string       // aguments for the systemtap script
	Class     SystemTapClass // system tap class
}

// GetID implement Nemesis
func (s *SystemTap) GetID() string {
	return s.P.ID
}

// GetType implement Nemesis
func (s *SystemTap) GetType() int {
	return s.P.Type
}

// GetStatus implement Nemesis
func (s *SystemTap) GetStatus() int {
	return s.P.Status
}

func newSTPIOError(argument map[string]string) (*SystemTap, error) {
	percent := common.STPDefaultPercent
	rw := common.STPRW
	script := common.STPIOErrorScript
	// script path
	if v, ok := argument[common.STPScriptDir]; ok {
		script = v + "/" + script
	}
	var dir, pidfile string
	// dir limitation
	if v, ok := argument[common.STPDir]; ok {
		dir = v
	}
	// percent limitation
	if v, ok := argument[common.STPPercent]; ok {
		d, err := strconv.Atoi(v)
		if err != nil || d < 0 || d >= 100 {
			glog.Warningf("[Systemtap] unexpected percent %s, use default value: %s", v, common.STPDefaultPercent)
		} else {
			percent = v
		}
	}
	// rw limitation
	flag := 0
	if v, ok := argument[common.STPRW]; ok {
		for _, b := range v {
			if b == 'r' {
				flag |= 0x001
			}
			if b == 'w' {
				flag |= 0x002
			}
		}
	}
	if flag == 1 {
		rw = "r"
	} else if flag == 2 {
		rw = "w"
	}
	// read pid
	if v, ok := argument[common.STPPidFile]; ok {
		pidfile = v
	}
	pid, err := util.GetPidFromfile(pidfile)
	if err != nil {
		glog.Warningf("[Systemtap] get pid from file %s failed: %v", pidfile, err)
		return nil, err
	}

	// create instance
	s := &SystemTap{
		Arguments: []string{
			common.STPBin,
			common.STPNoOverload,
			script,
			common.STPGuru,
			common.STPPid,
			pid,
			rw,
			percent,
			dir,
		},
		Class: SystemTapIOError,
	}
	// error_code
	if v, ok := argument[common.STPWriteErrorCode]; ok {
		if _, err := strconv.Atoi(v); err != nil {
			glog.Warningf("[Systemtap] unexpected write error code: %s", v)
			return nil, fmt.Errorf("unexpected write error code: %s", v)
		}
		s.Arguments = append(s.Arguments, v)
	}
	if v, ok := argument[common.STPReadErrorCode]; ok {
		if _, err := strconv.Atoi(v); err != nil {
			glog.Warningf("[Systemtap] unexpected read error code: %s", v)
			return nil, fmt.Errorf("unexpected read error code: %s", v)
		}
		s.Arguments = append(s.Arguments, v)
	}
	// create command
	s.P = NewPersistence(strings.Join(s.Arguments, " "))
	s.P.Type = common.TaskIOError
	return s, nil
}

func newSTPIODelay(argument map[string]string) (*SystemTap, error) {
	rw := common.STPDefaultRW
	delay := common.STPDefaultDelayUs
	maxDelay := common.STPDefaultMaxDelayUs
	script := common.STPIODelayScript
	// script path
	if v, ok := argument[common.STPScriptDir]; ok {
		script = v + "/" + script
	}
	// dir limitation
	var dir, pidfile string
	if v, ok := argument[common.STPDir]; ok {
		dir = v
	}
	// rw limitation
	flag := 0
	if v, ok := argument[common.STPRW]; ok {
		for _, b := range v {
			if b == 'r' {
				flag |= 0x001
			}
			if b == 'w' {
				flag |= 0x002
			}
		}
	}
	if flag == 1 {
		rw = "r"
	} else if flag == 2 {
		rw = "w"
	}
	// delay
	if v, ok := argument[common.STPDelay]; ok {
		d, err := strconv.Atoi(v)
		if err != nil || d < 0 {
			glog.Warningf("[Systemtap] unexpected delay: %s", v)
		} else {
			delay = v
		}
	}
	// max delay
	if v, ok := argument[common.STPMaxDelay]; ok {
		d, err := strconv.Atoi(v)
		if err != nil || d < 0 {
			glog.Warningf("[Systemtap] unexpected max delay: %s", v)
		} else {
			maxDelay = v
		}
	}
	// read pid
	if v, ok := argument[common.STPPidFile]; ok {
		pidfile = v
	}
	pid, err := util.GetPidFromfile(pidfile)
	if err != nil {
		glog.Warningf("[Systemtap] get pid from file %s failed: %v", pidfile, err)
		return nil, err
	}
	// create instance
	s := &SystemTap{
		Arguments: []string{
			common.STPBin,
			script,
			common.STPGuru,
			common.STPPid,
			pid,
			rw,
			delay,
			maxDelay,
			dir,
		},
		Class: SystemTapIODelay,
	}
	// create command
	s.P = NewPersistence(strings.Join(s.Arguments, " "))
	s.P.Type = common.TaskIODelay
	return s, nil
}

// NewSTP create a new systemtap runtime
func NewSTP(argument map[string]string, class SystemTapClass) (*SystemTap, error) {
	switch SystemTapClass(class) {
	case SystemTapIOError:
		return newSTPIOError(argument)
	case SystemTapIODelay:
		return newSTPIODelay(argument)
	default:
		glog.Warningf("[Systemtap] unknown script class %d", class)
	}
	return nil, fmt.Errorf("unknown systemtap script class %d", class)
}

// Run start the systemtap probe
func (s *SystemTap) Run() error {
	return s.P.Run()
}

// Terminate stop the systemtap probe
func (s *SystemTap) Terminate() error {
	return s.P.Terminate()
}
