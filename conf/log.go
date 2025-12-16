package conf

type LogConfig struct {
	Level  string `json:"Level"`
	Output string `json:"Output"`
	Pprof  string `json:"Pprof"` // pprof HTTP 监听地址，如 "127.0.0.1:6060"
}
