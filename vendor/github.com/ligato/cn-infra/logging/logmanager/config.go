package logmanager

// NewConf creates default configuration with InfoLevel & empty loggers.
// Suitable also for usage in flavor to programmatically specify default behavior.
func NewConf() *Config {
	return &Config{
		DefaultLevel: "",
		Loggers:      []LoggerConfig{},
		Hooks:        make(map[string]HookConfig),
	}
}

// Config is a binding that supports to define default log levels for multiple loggers
type Config struct {
	DefaultLevel string                `json:"default-level"`
	Loggers      []LoggerConfig        `json:"loggers"`
	Hooks        map[string]HookConfig `json:"hooks"`
}

// LoggerConfig is configuration of a particular logger.
// Currently we support only logger level.
type LoggerConfig struct {
	Name  string
	Level string //debug, info, warn, error, fatal, panic
}

// HookConfig contains configuration of hook services
type HookConfig struct {
	Protocol string
	Address  string
	Port     int
	Levels   []string
}
