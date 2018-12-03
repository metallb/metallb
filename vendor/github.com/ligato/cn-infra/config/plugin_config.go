package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/namsral/flag"

	"github.com/ligato/cn-infra/logging/logrus"
)

const (
	// FlagSuffix is added to plugin name while loading plugins configuration.
	FlagSuffix = "-config"

	// EnvSuffix is added to plugin name while loading plugins configuration from ENV variable.
	EnvSuffix = "_CONFIG"

	// FileExtension is used as a default extension for config files in flags.
	FileExtension = ".conf"
)

// FlagName returns config flag name for the name, usually plugin.
func FlagName(name string) string {
	return strings.ToLower(name) + FlagSuffix
}

// Filename returns config filename for the name, usually plugin.
func Filename(name string) string {
	return name + FileExtension
}

// EnvVar returns config env variable for the name, usually plugin.
func EnvVar(name string) string {
	return strings.ToUpper(name) + EnvSuffix
}

const (
	// DirFlag as flag name (see implementation in declareFlags())
	// is used to define default directory where config files reside.
	// This flag name is derived from the name of the plugin.
	DirFlag = "config-dir"

	// DirDefault holds a default value "." for flag, which represents current working directory.
	DirDefault = "."

	// DirUsage used as a flag (see implementation in declareFlags()).
	DirUsage = "Location of the config files; can also be set via 'CONFIG_DIR' env variable."
)

// DefineDirFlag defines flag for configuration directory.
func DefineDirFlag() {
	if flag.CommandLine.Lookup(DirFlag) == nil {
		flag.CommandLine.String(DirFlag, DirDefault, DirUsage)
	}
}

// PluginConfig is API for plugins to access configuration.
//
// Aim of this API is to let a particular plugin to bind it's configuration
// without knowing a particular key name. The key name is injected into Plugin.
type PluginConfig interface {
	// LoadValue parses configuration for a plugin and stores the results in data.
	// The argument data is a pointer to an instance of a go structure.
	LoadValue(data interface{}) (found bool, err error)

	// GetConfigName returns config name derived from plugin name:
	// flag = PluginName + FlagSuffix (evaluated most often as absolute path to a config file)
	GetConfigName() string
}

// FlagSet is a type alias for flag.FlagSet.
type FlagSet = flag.FlagSet

// pluginFlags is used for storing flags for Plugins before agent starts.
var pluginFlags = make(map[string]*FlagSet)

// DefineFlagsFor registers defined flags for plugin with given name.
func DefineFlagsFor(name string) {
	if plugSet, ok := pluginFlags[name]; ok {
		plugSet.VisitAll(func(f *flag.Flag) {
			flag.CommandLine.Var(f.Value, f.Name, f.Usage)
		})
	}
}

type options struct {
	FlagName    string
	FlagDefault string
	FlagUsage   string

	flagSet *FlagSet
}

// Option is an option used in ForPlugin
type Option func(*options)

// WithCustomizedFlag is an option to customize config flag for plugin in ForPlugin.
// The parameters are used to replace defaults in this order: flag name, default, usage.
func WithCustomizedFlag(s ...string) Option {
	return func(o *options) {
		if len(s) > 0 {
			o.FlagName = s[0]
		}
		if len(s) > 1 {
			o.FlagDefault = s[1]
		}
		if len(s) > 2 {
			o.FlagUsage = s[2]
		}
	}
}

// WithExtraFlags is an option to define additional flags for plugin in ForPlugin.
func WithExtraFlags(f func(flags *FlagSet)) Option {
	return func(o *options) {
		f(o.flagSet)
	}
}

// ForPlugin returns API that is injectable to a particular Plugin
// and is used to read it's configuration.
//
// By default it tries to lookup `<plugin-name> + "-config"`in flags and declare
// the flag if it's not defined yet. There are options that can be used
// to customize the config flag for plugin and/or define additional flags for the plugin.
func ForPlugin(name string, opts ...Option) PluginConfig {
	opt := options{
		FlagName:    FlagName(name),
		FlagDefault: Filename(name),
		FlagUsage: fmt.Sprintf("Location of the %q plugin config file; can also be set via %q env variable.",
			name, EnvVar(name)),
		flagSet: flag.NewFlagSet(name, flag.ExitOnError),
	}
	for _, o := range opts {
		o(&opt)
	}

	if opt.FlagName != "" && opt.flagSet.Lookup(opt.FlagName) == nil {
		opt.flagSet.String(opt.FlagName, opt.FlagDefault, opt.FlagUsage)
	}

	pluginFlags[name] = opt.flagSet

	return &pluginConfig{
		configFlag: opt.FlagName,
	}
}

// Dir returns config directory by evaluating the flag DirFlag. It interprets "." as current working directory.
func Dir() (dir string, err error) {
	if flg := flag.CommandLine.Lookup(DirFlag); flg != nil {
		val := flg.Value.String()
		if strings.HasPrefix(val, ".") {
			cwd, err := os.Getwd()
			if err != nil {
				return cwd, err
			}
			if len(val) > 1 {
				return filepath.Join(cwd, val[1:]), nil
			}
			return cwd, nil
		}
		return val, nil
	}
	return "", nil
}

type pluginConfig struct {
	configFlag string
	access     sync.Mutex
	configName string
}

// LoadValue binds the configuration to config method argument.
func (p *pluginConfig) LoadValue(config interface{}) (found bool, err error) {
	cfgName := p.GetConfigName()
	if cfgName == "" {
		return false, nil
	}

	// TODO: switch to Viper (possible to have one huge config file)
	err = ParseConfigFromYamlFile(cfgName, config)
	if err != nil {
		return false, err
	}

	return true, nil
}

// GetConfigName looks up flag value and uses it to:
// 1. Find config in flag value location.
// 2. Alternatively, it tries to find it in config dir
// (see also Dir() comments).
func (p *pluginConfig) GetConfigName() string {
	p.access.Lock()
	defer p.access.Unlock()
	if p.configName == "" {
		p.configName = p.getConfigName()
	}
	return p.configName
}

func (p *pluginConfig) getConfigName() string {
	if flg := flag.CommandLine.Lookup(p.configFlag); flg != nil {
		if val := flg.Value.String(); val != "" {
			// if the file exists (value from flag)
			if _, err := os.Stat(val); !os.IsNotExist(err) {
				return val
			}
			cfgDir, err := Dir()
			if err != nil {
				logrus.DefaultLogger().Error(err)
				return ""
			}
			// if the file exists (flag value in config dir)
			dirVal := filepath.Join(cfgDir, val)
			if _, err := os.Stat(dirVal); !os.IsNotExist(err) {
				return dirVal
			}
		}
	}
	return ""
}
