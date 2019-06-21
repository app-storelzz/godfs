package common

const (
	VERSION          = "2.0.0"
	CLIENT  BootMode = 0
	STORAGE BootMode = 1
	TRACKER BootMode = 2
)

type BootMode uint32

type StorageConfig struct {
	Trackers              []string `json:"trackers"`
	Secret                string   `json:"secret"`
	Group                 string   `json:"group"`
	InstanceId            string   `json:"instanceId"`
	BindAddress           string   `json:"bindAddress"`
	Port                  int      `json:"port"`
	AdvertiseAddress      string   `json:"advertiseAddress"`
	AdvertisePort         int      `json:"advertisePort"`
	DataDir               string   `json:"dataDir"`
	PreferredNetworks     string   `json:"preferredNetworks"`
	LogLevel              string   `json:"logLevel"`
	LogDir                string   `json:"logDir"`
	SaveLog2File          bool     `json:"saveLog2File"`
	MaxRollingLogfileSize int      `json:"maxRollingLogfileSize"`
	LogRotationInterval   string   `json:"logRotationInterval"`
	EnableHttp            bool     `json:"enableHttp"`
	HttpPort              int      `json:"httpPort"`
	HttpAuth              string   `json:"httpAuth"`
	EnableMimeTypes       bool     `json:"enableMimeTypes"`
	AllowedDomains        []string `json:"allowedDomains"`
}

type TrackerConfig struct {
	Trackers              []string `json:"trackers"`
	Secret                string   `json:"secret"`
	InstanceId            string   `json:"instanceId"`
	BindAddress           string   `json:"bindAddress"`
	Port                  int      `json:"port"`
	AdvertiseAddress      string   `json:"advertiseAddress"`
	AdvertisePort         int      `json:"advertisePort"`
	DataDir               string   `json:"dataDir"`
	PreferredNetworks     string   `json:"preferredNetworks"`
	LogLevel              string   `json:"logLevel"`
	LogDir                string   `json:"logDir"`
	SaveLog2File          bool     `json:"saveLog2File"`
	MaxRollingLogfileSize int      `json:"maxRollingLogfileSize"`
	LogRotationInterval   string   `json:"logRotationInterval"`
	EnableHttp            bool     `json:"enableHttp"`
	HttpPort              int      `json:"httpPort"`
	HttpAuth              string   `json:"httpAuth"`
	EnableMimeTypes       bool     `json:"enableMimeTypes"`
	AllowedDomains        []string `json:"allowedDomains"`
}

type ClientConfig struct {
	Trackers []string `json:"trackers"`
	Storages []string `json:"storages"`
	LogLevel string   `json:"logLevel"`
	Secret   string   `json:"secret"`
}