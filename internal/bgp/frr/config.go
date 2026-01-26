// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	"errors"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"go.universe.tf/metallb/internal/bgp/community"
	"go.universe.tf/metallb/internal/ipfamily"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	configFileName     = "/etc/frr_reloader/frr.conf"
	reloaderSocketName = "/etc/frr_reloader/frr-reloader.sock"
)

var (
	//go:embed templates/* templates/*
	templates              embed.FS
	reloadRequestIDCounter atomic.Uint64
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

type reloadRequest struct {
	Action string `json:"action"`
	ID     int    `json:"id"`
}

type reloadResponse struct {
	ID     int  `json:"id"`
	Result bool `json:"result"`
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
func writeConfig(config string, filename string, mu *sync.Mutex) error {
	mu.Lock()
	defer mu.Unlock()
	return os.WriteFile(filename, []byte(config), 0600)
}

// reloadConfig requests that FRR reloads the configuration file. This is
// called after updating the configuration.
var reloadConfig = func() error {
	socketPath, found := os.LookupEnv("FRR_RELOADER_SOCKET")
	if !found {
		socketPath = reloaderSocketName
	}

	// Generate unique request ID
	requestID := int(reloadRequestIDCounter.Add(1))

	// Create the reload request
	request := reloadRequest{
		Action: "reload",
		ID:     requestID,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal reload request: %w", err)
	}

	// Create HTTP client with Unix socket transport
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
		Timeout: 30 * time.Second,
	}

	// Make the HTTP POST request
	resp, err := client.Post("http://unix/", "application/json", bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("failed to send reload request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("reload request failed with status: %d", resp.StatusCode)
	}

	// Parse the response
	var response reloadResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode reload response: %w", err)
	}

	// Verify the response ID matches
	if response.ID != requestID {
		return fmt.Errorf("response ID mismatch: expected %d, got %d", requestID, response.ID)
	}

	// Check if the reload was successful
	if !response.Result {
		return fmt.Errorf("FRR reload failed")
	}

	return nil
}

// generateAndReloadConfigFile takes a 'struct frrConfig' and, using a template,
// generates and writes a valid FRR configuration file. If this completes
// successfully it will also force FRR to reload that configuration file.
func generateAndReloadConfigFile(config *frrConfig, l log.Logger, filename string, fileLock *sync.Mutex) error {
	configString, err := templateConfig(config)
	if err != nil {
		level.Error(l).Log("op", "reload", "error", err, "cause", "template", "config", config)
		return err
	}
	err = writeConfig(configString, filename, fileLock)
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
// the update requests are sent, and pools any requests coming in during a reload.
// When a reload is in progress, all incoming configs are pooled, and the latest
// one will be reloaded after the current reload completes.
func debouncer(body func(config *frrConfig) error,
	reload <-chan reloadEvent,
	_ time.Duration,
	failureRetryInterval time.Duration,
	l log.Logger) {
	var configToUse atomic.Pointer[frrConfig]
	triggerReload := make(chan struct{}, 1)

	// Reload goroutine: waits for trigger signal, loads latest config atomically, and reloads.
	// On failure, waits for retry interval then triggers a retry (unless already triggered).
	go func() {
		for {
			<-triggerReload
			cfg := configToUse.Load()
			level.Debug(l).Log("op", "reload", "action", "start reload")
			err := body(cfg)
			if err != nil {
				level.Error(l).Log("op", "reload", "error", err, "action", "retry after interval")
				time.Sleep(failureRetryInterval)

				// Trigger retry if not already pending (non-blocking).
				select {
				case triggerReload <- struct{}{}:
				default:
				}
			}
		}
	}()

	// Main event loop: receives reload events and triggers reloads.
	// The config is stored atomically first, then a trigger is sent (non-blocking).
	// This ordering guarantees the reload goroutine always sees the latest config.
	go func() {
		for newCfg := range reload {
			config := configToUse.Load()

			if newCfg.useOld && config == nil {
				level.Debug(l).Log("op", "reload", "action", "ignore config", "reason", "nil config")
				continue // just ignore the event
			}
			if !newCfg.useOld && reflect.DeepEqual(newCfg.config, configToUse.Load()) {
				level.Debug(l).Log("op", "reload", "action", "ignore config", "reason", "same config")
				continue // config hasn't changed
			}
			if !newCfg.useOld {
				configToUse.Store(newCfg.config)
			}

			// Store new config atomically, then trigger reload if not already pending.
			select {
			case triggerReload <- struct{}{}:
				// Successfully triggered reload
			default:
				// Reload already pending - the reload goroutine will pick up our latest config
			}
		}
	}()
}
