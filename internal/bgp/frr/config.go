// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"syscall"
	"text/template"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/pkg/errors"
	"go.universe.tf/metallb/internal/ipfamily"
)

var (
	configFileName      = "/etc/frr_reloader/frr.conf"
	reloaderPidFileName = "/etc/frr_reloader/reloader.pid"
	//go:embed templates/* templates/*
	templates embed.FS
)

type frrConfig struct {
	Loglevel    string
	Hostname    string
	Routers     []*routerConfig
	BFDProfiles []BFDProfile
	ExtraConfig string
}

type reloadEvent struct {
	config *frrConfig
	useOld bool
}

// TODO: having global prefix lists works only because we advertise all the addresses
// to all the neighbors. Once this constraint is changed, we may need prefix-lists per neighbor.

type routerConfig struct {
	MyASN        uint32
	RouterID     string
	Neighbors    []*neighborConfig
	VRF          string
	IPV4Prefixes []string
	IPV6Prefixes []string
}

type BFDProfile struct {
	Name             string
	ReceiveInterval  *uint32
	TransmitInterval *uint32
	DetectMultiplier *uint32
	EchoInterval     *uint32
	EchoMode         bool
	PassiveMode      bool
	MinimumTTL       *uint32
}

type neighborConfig struct {
	IPFamily            ipfamily.Family
	Name                string
	ASN                 uint32
	Addr                string
	SrcAddr             string
	Port                uint16
	HoldTime            uint64
	KeepaliveTime       uint64
	Password            string
	Advertisements      []*advertisementConfig
	BFDProfile          string
	EBGPMultiHop        bool
	VRFName             string
	HasV4Advertisements bool
	HasV6Advertisements bool
}

func (n *neighborConfig) ID() string {
	if n.VRFName == "" {
		return n.Addr
	}
	return fmt.Sprintf("%s-%s", n.Addr, n.VRFName)
}

type advertisementConfig struct {
	IPFamily         ipfamily.Family
	Prefix           string
	Communities      []string
	LargeCommunities []string
	LocalPref        uint32
}

// routerName() defines the format of the key of the "Routers" map in the
// frrConfig struct.
func routerName(srcAddr string, myASN uint32, vrfName string) string {
	return fmt.Sprintf("%d@%s@%s", myASN, srcAddr, vrfName)
}

// neighborName() defines the format of key of the 'Neighbors' map in the
// routerConfig struct.
func neighborName(peerAddr string, ASN uint32, vrfName string) string {
	return fmt.Sprintf("%d@%s@%s", ASN, peerAddr, vrfName)
}

// templateConfig uses the template library to template
// 'globalConfigTemplate' using 'data'.
func templateConfig(data interface{}) (string, error) {
	counterMap := map[string]int{}
	t, err := template.New("frr.tmpl").Funcs(
		template.FuncMap{
			"counter": func(counterName string) int {
				counter := counterMap[counterName]
				counter++
				counterMap[counterName] = counter
				return counter
			},
			"frrIPFamily": func(ipFamily ipfamily.Family) string {
				if ipFamily == "ipv6" {
					return "ipv6"
				}
				return "ip"
			},
			"localPrefPrefixList": func(neighbor *neighborConfig, localPreference uint32) string {
				return fmt.Sprintf("%s-%d-%s-localpref-prefixes", neighbor.ID(), localPreference, neighbor.IPFamily)
			},
			"communityPrefixList": func(neighbor *neighborConfig, community string) string {
				return fmt.Sprintf("%s-%s-%s-community-prefixes", neighbor.ID(), community, neighbor.IPFamily)
			},
			"largeCommunityPrefixList": func(neighbor *neighborConfig, community string) string {
				return fmt.Sprintf("%s-large:%s-%s-community-prefixes", neighbor.ID(), community, neighbor.IPFamily)
			},
			"allowedPrefixList": func(neighbor *neighborConfig) string {
				return fmt.Sprintf("%s-pl-%s", neighbor.ID(), neighbor.IPFamily)
			},
			"mustDisableConnectedCheck": func(ipFamily ipfamily.Family, myASN, asn uint32, eBGPMultiHop bool) bool {
				// return true only for IPv6 eBGP sessions
				if ipFamily == "ipv6" && myASN != asn && !eBGPMultiHop {
					return true
				}
				return false
			},
			"dict": func(values ...interface{}) (map[string]interface{}, error) {
				if len(values)%2 != 0 {
					return nil, errors.New("invalid dict call, expecting even number of args")
				}
				dict := make(map[string]interface{}, len(values)/2)
				for i := 0; i < len(values); i += 2 {
					key, ok := values[i].(string)
					if !ok {
						return nil, fmt.Errorf("dict keys must be strings, got %v %T", values[i], values[i])
					}
					dict[key] = values[i+1]
				}
				return dict, nil
			},
		}).ParseFS(templates, "templates/*")
	if err != nil {
		return "", err
	}

	var b bytes.Buffer
	err = t.Execute(&b, data)
	return b.String(), err
}

// writeConfigFile writes the FRR configuration file (represented as a string)
// to 'filename'.
func writeConfig(config string, filename string) error {
	return os.WriteFile(filename, []byte(config), 0600)
}

// reloadConfig requests that FRR reloads the configuration file. This is
// called after updating the configuration.
var reloadConfig = func() error {
	pidFile, found := os.LookupEnv("FRR_RELOADER_PID_FILE")
	if found {
		reloaderPidFileName = pidFile
	}

	pid, err := os.ReadFile(reloaderPidFileName)
	if err != nil {
		return err
	}

	pidInt, err := strconv.Atoi(string(pid))
	if err != nil {
		return err
	}

	// send HUP signal to FRR reloader
	err = syscall.Kill(pidInt, syscall.SIGHUP)
	if err != nil {
		return err
	}

	return nil
}

// generateAndReloadConfigFile takes a 'struct frrConfig' and, using a template,
// generates and writes a valid FRR configuration file. If this completes
// successfully it will also force FRR to reload that configuration file.
func generateAndReloadConfigFile(config *frrConfig, l log.Logger) error {
	filename, found := os.LookupEnv("FRR_CONFIG_FILE")
	if found {
		configFileName = filename
	}

	configString, err := templateConfig(config)
	if err != nil {
		level.Error(l).Log("op", "reload", "error", err, "cause", "template", "config", config)
		return err
	}
	err = writeConfig(configString, configFileName)
	if err != nil {
		level.Error(l).Log("op", "reload", "error", err, "cause", "writeConfig", "config", config)
		return err
	}

	err = reloadConfig()
	if err != nil {
		level.Error(l).Log("op", "reload", "error", err, "cause", "reload", "config", config)
		return err
	}
	return nil
}

// debouncer takes a function that processes an frrConfig, a channel where
// the update requests are sent, and squashes any requests coming in a given timeframe
// as a single request.
func debouncer(body func(config *frrConfig) error,
	reload <-chan reloadEvent,
	reloadInterval time.Duration,
	failureRetryInterval time.Duration,
	l log.Logger) {
	go func() {
		var config *frrConfig
		var timeOut <-chan time.Time
		timerSet := false
		for {
			select {
			case newCfg, ok := <-reload:
				if !ok { // the channel was closed
					return
				}
				if newCfg.useOld && config == nil {
					level.Debug(l).Log("op", "reload", "action", "ignore config", "reason", "nil config")
					continue // just ignore the event
				}
				if !newCfg.useOld && reflect.DeepEqual(newCfg.config, config) {
					level.Debug(l).Log("op", "reload", "action", "ignore config", "reason", "same config")
					continue // config hasn't changed
				}
				if !newCfg.useOld {
					config = newCfg.config
				}
				if !timerSet {
					timeOut = time.After(reloadInterval)
					timerSet = true
				}
			case <-timeOut:
				err := body(config)
				if err != nil {
					timeOut = time.After(failureRetryInterval)
					timerSet = true
					continue
				}
				timerSet = false
			}
		}
	}()
}
