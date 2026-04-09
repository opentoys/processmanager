package utils

// 内存单位常量
const (
	_          = iota // ignore first value by assigning to blank identifier
	KB float64 = 1 << (10 * iota)
	MB
	GB
	TB
)

// 时间单位常量
const (
	Day    = 24 * 60 * 60
	Hour   = 60 * 60
	Minute = 60
)

var (
	ProcessManagerName = "pm"
)

const (
	ProcessStatusRunning = "running"
	ProcessStatusStopped = "stopped"
)

const (
	PMSaveFile   = "pm.save"
	PMStateFile  = "pm.state"
	PMSocketFile = "pm.sock"
	PMPidFile    = "pm.pid"
	PMLogDir     = "logs/"
	PMConfigFile = "config.json"
)

const (
	PMENV_WORKSPACE = "PM_WORKSPACE"
)

// 版本信息，通过 -ldflags 在编译时注入
var (
	Version   = "dev"
	GoVersion = "unknown"
)
