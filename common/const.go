package common

import "time"

// Task type
const (
	TaskEmpty = iota
	TaskPartition
	TaskService
	TaskIOError
	TaskIODelay
)

// key of Arguments
const (
	ArgumentsKeyNodes     = "nodes"
	ArgumentsKeyRun       = "run"
	ArgumentsKeyTerminate = "terminate"
	ArgumentsSeparator    = ","
	ArgumentsDebug        = "debug"
	ArgumentsKeyCommand   = "command"
	ArgumentsKeyTarget    = "target"
)

// Task operations
const (
	OperationStart = 1 << iota
	OperationStop
	OperationClean
)

// Task const
const (
	TickerDefaultDelay = time.Second * 5
)

// TaskType return task type by string
func TaskType(i int) string {
	switch i {
	case TaskEmpty:
		return "Empty"
	case TaskService:
		return "Service"
	case TaskPartition:
		return "Partition"
	case TaskIOError:
		return "IOError"
	case TaskIODelay:
		return "IODelay"
	default:
		return "Unknown"
	}
}

// systemtap
const (
	STPScriptDir     = "stp_script_dir"
	STPIOErrorScript = "file_io_error_vfs.stp"
	STPIODelayScript = "file_io_delay_vfs.stp"
	STPLib           = "lib"
	STPBin           = "stap"
	STPNoOverload    = "-DSTP_NO_OVERLOAD"
	STPGuru          = "-g"
	STPPid           = "-x"
	STPMaxSkipped    = "-DMAXSKIPPED"
	STPLink          = "-I"

	STPClass          = "class"
	STPRW             = "rw"
	STPPercent        = "percent"
	STPDir            = "dir"
	STPPidFile        = "pidfile"
	STPWriteErrorCode = "write_error_code"
	STPReadErrorCode  = "read_error_code"
	STPDelay          = "delay"
	STPMaxDelay       = "max_delay"

	STPDefaultPercent    = "5"
	STPDefaultRW         = "rw"
	STPDefaultDelayUs    = "300"
	STPDefaultMaxDelayUs = "0"
	STPDefaultMaxSkipped = "9999"
)
