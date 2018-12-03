package logmanager

import (
	"fmt"
	"log/syslog"
	"strconv"

	"github.com/bshuster-repo/logrus-logstash-hook"
	"github.com/evalphobia/logrus_fluent"
	"github.com/sirupsen/logrus"
	lgSyslog "github.com/sirupsen/logrus/hooks/syslog"
)

// list of supported hook services to send errors to remote syslog server
const (
	HookSysLog   = "syslog"
	HookLogStash = "logstash"
	HookFluent   = "fluent"
)

// commonHook implements that Hook with own level definition
type commonHook struct {
	logrus.Hook
	levels []logrus.Level
}

// Levels overrides implementation from embedded interface
func (cH *commonHook) Levels() []logrus.Level {
	return cH.levels
}

// store hook into registy for late use and applies to existing loggers
func (p *Plugin) addHook(hookName string, hookConfig HookConfig) error {
	var lgHook logrus.Hook
	var err error

	switch hookName {
	case HookSysLog:
		address := hookConfig.Address
		if hookConfig.Address != "" {
			address = address + ":" + strconv.Itoa(hookConfig.Port)
		}
		lgHook, err = lgSyslog.NewSyslogHook(
			hookConfig.Protocol,
			address,
			syslog.LOG_INFO,
			p.ServiceLabel.GetAgentLabel(),
		)
	case HookLogStash:
		lgHook, err = logrustash.NewHook(
			hookConfig.Protocol,
			hookConfig.Address+":"+strconv.Itoa(hookConfig.Port),
			p.ServiceLabel.GetAgentLabel(),
		)
	case HookFluent:
		lgHook, err = logrus_fluent.NewWithConfig(logrus_fluent.Config{
			Host:       hookConfig.Address,
			Port:       hookConfig.Port,
			DefaultTag: p.ServiceLabel.GetAgentLabel(),
		})
	default:
		return fmt.Errorf("unsupported hook: %q", hookName)
	}
	if err != nil {
		return fmt.Errorf("creating hook for %v failed: %v", hookName, err)
	}
	// create hook
	cHook := &commonHook{Hook: lgHook}
	// fill up defined levels, or use default if not defined
	if len(hookConfig.Levels) == 0 {
		cHook.levels = []logrus.Level{logrus.PanicLevel, logrus.FatalLevel, logrus.ErrorLevel}
	} else {
		for _, level := range hookConfig.Levels {
			if lgl, err := logrus.ParseLevel(level); err == nil {
				cHook.levels = append(cHook.levels, lgl)
			} else {
				p.Log.Warnf("cannot parse hook log level %v : %v", level, err.Error())
			}
		}
	}
	// add hook to existing loggers and store it into registry for late use
	p.LogRegistry.AddHook(cHook)
	return nil
}
