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

	"errors"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"go.universe.tf/metallb/internal/bgp/community"
	"go.universe.tf/metallb/internal/ipfamily"
	"k8s.io/apimachinery/pkg/util/sets"
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
	IPFamily                 ipfamily.Family
	Name                     string
	ASN                      string
	Addr                     string
	Iface                    string
	SrcAddr                  string
	Port                     uint16
	HoldTime                 *int64
	KeepaliveTime            *int64
	ConnectTime              int64
	Password                 string
	BFDProfile               string
	GracefulRestart          bool
	EBGPMultiHop             bool
	VRFName                  string
	PrefixesV4               []string
	PrefixesV6               []string
	prefixesV4Set            sets.Set[string]
	prefixesV6Set            sets.Set[string]
	CommunityPrefixModifiers map[string]CommunityPrefixList
	LocalPrefPrefixModifiers map[string]LocalPrefPrefixList
}

func (n *neighborConfig) ID() string {
	id := n.Addr
	if n.Iface != "" {
		id = n.Iface
	}
	vrf := ""
	if n.VRFName != "" {
		vrf = "-" + n.VRFName
	}
	return id + vrf
}

func (n *neighborConfig) CommunityPrefixLists() []CommunityPrefixList {
	return sortMap(n.CommunityPrefixModifiers)
}

func (n *neighborConfig) LocalPrefPrefixLists() []LocalPrefPrefixList {
	return sortMap(n.LocalPrefPrefixModifiers)
}

func (n *neighborConfig) ToAdvertisePrefixListV4() string {
	return fmt.Sprintf("%s-allowed-%s", n.ID(), "ipv4")
}

func (n *neighborConfig) ToAdvertisePrefixListV6() string {
	return fmt.Sprintf("%s-allowed-%s", n.ID(), "ipv6")
}

type PropertyPrefixList struct {
	Name        string
	IPFamily    string
	prefixesSet sets.Set[string]
	Prefixes    []string
}

type CommunityPrefixList struct {
	PropertyPrefixList
	Community community.BGPCommunity
}

func (c CommunityPrefixList) SetStatement() string {
	if community.IsLarge(c.Community) {
		return fmt.Sprintf("set large-community %s additive", c.Community.String())
	}
	return fmt.Sprintf("set community %s additive", c.Community.String())
}

type LocalPrefPrefixList struct {
	PropertyPrefixList
	LocalPreference uint32
}

func (l LocalPrefPrefixList) SetStatement() string {
	return fmt.Sprintf("set local-preference %d", l.LocalPreference)
}

// RouterName() defines the format of the key of the "Routers" map in the
// frrConfig struct.
func RouterName(srcAddr string, myASN uint32, vrfName string) string {
	return fmt.Sprintf("%d@%s@%s", myASN, srcAddr, vrfName)
}

// neighborName() defines the format of key of the 'Neighbors' map in the
// routerConfig struct.
func NeighborName(peerAddr, iface string, ASN uint32, dynamicASN string, vrfName string) string {
	asn := asnFor(ASN, dynamicASN)
	if peerAddr == "" {
		return fmt.Sprintf("%s@%s@%s", asn, iface, vrfName)
	}
	return fmt.Sprintf("%s@%s@%s", asn, peerAddr, vrfName)
}

func asnFor(ASN uint32, dynamicASN string) string {
	asn := strconv.FormatUint(uint64(ASN), 10)
	if dynamicASN != "" {
		asn = dynamicASN
	}
	return asn
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
			"activateNeighborFor": func(ipFamily string, neighbourFamily ipfamily.Family) bool {
				return neighbourFamily.String() == ipFamily || neighbourFamily == ipfamily.DualStack
			},
			"allowedPrefixList": func(neighbor *neighborConfig, ipFamily string) string {
				return fmt.Sprintf("%s-pl-%s", neighbor.ID(), ipFamily)
			},
			"mustDisableConnectedCheck": func(ipFamily ipfamily.Family, myASN uint32, asn, iface string, eBGPMultiHop bool) bool {
				// return true only for non-multihop IPv6 eBGP sessions

				if ipFamily != ipfamily.IPv6 {
					return false
				}

				if eBGPMultiHop {
					return false
				}

				if iface != "" {
					return true
				}

				// internal means we expect the session to be iBGP
				if asn == "internal" {
					return false
				}

				// external means we expect the session to be eBGP
				if asn == "external" {
					return true
				}

				// the peer's asn is not dynamic (it is a number),
				// we check if it is different than ours for eBGP
				if strconv.FormatUint(uint64(myASN), 10) != asn {
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

// writeConfig writes the FRR configuration file (represented as a string)
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
