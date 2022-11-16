// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"bytes"
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

var configFileName = "/etc/frr_reloader/frr.conf"
var reloaderPidFileName = "/etc/frr_reloader/reloader.pid"

// TODO: We will probably need to update this template when we start to
// integrate with FRR. The current template is a reasonable first pass
// and we can improve it in later commits.
//
// It may be necessary to arrange this into multiple nested templates
// (https://pkg.go.dev/text/template#hdr-Nested_template_definitions), this
// should also be considered.

const configTemplate = `
{{- define "localpreffilter" -}}
{{frrIPFamily .advertisement.IPFamily}} prefix-list {{localPrefPrefixList .neighbor .advertisement.LocalPref}} permit {{.advertisement.Prefix}}
route-map {{.neighbor.Addr}}-out permit {{counter .neighbor.Addr}}
  match {{frrIPFamily .advertisement.IPFamily}} address prefix-list {{localPrefPrefixList .neighbor .advertisement.LocalPref}}
  set local-preference {{.advertisement.LocalPref}}
  on-match next
{{- end -}}

{{- define "communityfilter" -}}
{{frrIPFamily .advertisement.IPFamily}} prefix-list {{communityPrefixList .neighbor .community}} permit {{.advertisement.Prefix}}
route-map {{.neighbor.Addr}}-out permit {{counter .neighbor.Addr}}
  match {{frrIPFamily .advertisement.IPFamily}} address prefix-list {{communityPrefixList .neighbor .community}}
  set community {{.community}} additive
  on-match next
{{- end -}}

{{- /* The prefixes are per router in FRR, but MetalLB api allows to associate a given BGPAdvertisement to a service IP,
     and a given advertisement contains both the properties of the announcement (i.e. community) and the list of peers
     we may want to advertise to. Because of this, for each neighbor we must opt-in and allow the advertisement, and
     deny all the others.*/ -}}
{{- define "neighborfilters" -}}

route-map {{.neighbor.Addr}}-in deny 20
{{- range $a := .neighbor.Advertisements }}
{{/* Advertisements for which we must enable set the local pref */}}
{{- if not (eq $a.LocalPref 0)}}
{{template "localpreffilter" dict "advertisement" $a "neighbor" $.neighbor}}
{{- end -}}

{{/* Advertisements for which we must enable the community property */}}
{{- range $c := $a.Communities }}
{{template "communityfilter" dict "advertisement" $a "neighbor" $.neighbor "community" $c}}
{{- end }}
{{/* this advertisement is allowed to the specific neighbor  */}}
{{frrIPFamily $a.IPFamily}} prefix-list {{allowedPrefixList $.neighbor}} permit {{$a.Prefix}}
{{- end }}

route-map {{$.neighbor.Addr}}-out permit {{counter $.neighbor.Addr}}
  match ip address prefix-list {{allowedPrefixList $.neighbor}}
route-map {{$.neighbor.Addr}}-out permit {{counter $.neighbor.Addr}}
  match ipv6 address prefix-list {{allowedPrefixList $.neighbor}}

ip prefix-list {{allowedPrefixList $.neighbor }} deny any
ipv6 prefix-list {{allowedPrefixList $.neighbor}} deny any
{{- end -}}

{{- define "neighborsession"}}
  neighbor {{.neighbor.Addr}} remote-as {{.neighbor.ASN}}
  {{- if .neighbor.EBGPMultiHop }}
  neighbor {{.neighbor.Addr}} ebgp-multihop
  {{- end }}
  {{ if .neighbor.Port -}}
  neighbor {{.neighbor.Addr}} port {{.neighbor.Port}}
  {{- end }}
  neighbor {{.neighbor.Addr}} timers {{.neighbor.KeepaliveTime}} {{.neighbor.HoldTime}}
  {{ if .neighbor.Password -}}
  neighbor {{.neighbor.Addr}} password {{.neighbor.Password}}
  {{- end }}
  {{ if .neighbor.SrcAddr -}}
  neighbor {{.neighbor.Addr}} update-source {{.neighbor.SrcAddr}}
  {{- end }}
{{- if ne .neighbor.BFDProfile ""}}
  neighbor {{.neighbor.Addr}} bfd profile {{.neighbor.BFDProfile}}
{{- end }}
{{- if  mustDisableConnectedCheck .neighbor.IPFamily .routerASN .neighbor.ASN .neighbor.EBGPMultiHop }}
  neighbor {{.neighbor.Addr}} disable-connected-check
{{- end }}
{{- end -}}

{{- define "neighborenableipfamily"}}
{{/* no bgp default ipv4-unicast prevents peering if no address families are defined. We declare an ipv4 one for the peer to make the pairing happen */}}
  address-family ipv4 unicast
    neighbor {{.Addr}} activate
    neighbor {{.Addr}} route-map {{.Addr}}-in in
    neighbor {{.Addr}} route-map {{.Addr}}-out out
  exit-address-family
  address-family ipv6 unicast
    neighbor {{.Addr}} activate
    neighbor {{.Addr}} route-map {{.Addr}}-in in
    neighbor {{.Addr}} route-map {{.Addr}}-out out
  exit-address-family
{{- end -}}

{{- define "bfdprofile" }}
  profile {{.profile.Name}}
    {{ if .profile.ReceiveInterval -}}
    receive-interval {{.profile.ReceiveInterval}}
    {{end -}}
    {{ if .profile.TransmitInterval -}}
    transmit-interval {{.profile.TransmitInterval}}
    {{end -}}
    {{ if .profile.DetectMultiplier -}}
    detect-multiplier {{.profile.DetectMultiplier}}
    {{end -}}
    {{ if .profile.EchoMode -}}
    echo-mode
    {{end -}}
    {{ if .profile.EchoInterval -}}
    echo-interval {{.profile.EchoInterval}}
    {{end -}}
    {{ if .profile.PassiveMode -}}
    passive-mode
    {{end -}}
    {{ if .profile.MinimumTTL -}}
    minimum-ttl {{ .profile.MinimumTTL }}
    {{end -}}
{{- end -}}

log file /etc/frr/frr.log {{.Loglevel}}
log timestamp precision 3
{{- if eq .Loglevel "debugging" }}
debug zebra events
debug zebra nht
debug zebra kernel
debug zebra rib
debug zebra nexthop
debug bgp neighbor-events
debug bgp updates
debug bgp keepalives
debug bgp nht
debug bgp zebra
debug bfd network
debug bfd peer
debug bfd zebra
{{- end }}
hostname {{.Hostname}}
ip nht resolve-via-default
ipv6 nht resolve-via-default

{{- range .Routers }}
{{- range .Neighbors }}
{{template "neighborfilters" dict "neighbor" .}}
{{- end }}
{{- end }}

{{range $r := .Routers -}}
router bgp {{$r.MyASN}}
  no bgp ebgp-requires-policy
  no bgp network import-check
  no bgp default ipv4-unicast
{{ if $r.RouterId }}
  bgp router-id {{$r.RouterId}}
{{- end }}

{{- range .Neighbors }}
{{- template "neighborsession" dict "neighbor" . "routerASN" $r.MyASN -}}
{{- end }}

{{- range $n := .Neighbors -}}
{{- template "neighborenableipfamily" . -}}
{{end -}}

{{- if gt (len .IPV4Prefixes) 0}}
  address-family ipv4 unicast
{{- range .IPV4Prefixes }}
    network {{.}}
{{- end}}
  exit-address-family
{{end }}

{{- if gt (len .IPV6Prefixes) 0}}
  address-family ipv6 unicast
{{- range .IPV6Prefixes }}
    network {{.}}
{{- end}}
  exit-address-family
{{end }}
{{end }}
{{- if gt (len .BFDProfiles) 0}}
bfd
{{- range .BFDProfiles }}
{{- template "bfdprofile" dict "profile" . -}}
{{- end }}
{{- end }}`

type frrConfig struct {
	Loglevel    string
	Hostname    string
	Routers     map[string]*routerConfig
	BFDProfiles []BFDProfile
}

type reloadEvent struct {
	config *frrConfig
	useOld bool
}

// TODO: having global prefix lists works only because we advertise all the addresses
// to all the neighbors. Once this constraint is changed, we may need prefix-lists per neighbor.

type routerConfig struct {
	MyASN        uint32
	RouterId     string
	Neighbors    map[string]*neighborConfig
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
	IPFamily       ipfamily.Family
	Name           string
	ASN            uint32
	Addr           string
	SrcAddr        string
	Port           uint16
	HoldTime       uint64
	KeepaliveTime  uint64
	Password       string
	Advertisements []*advertisementConfig
	BFDProfile     string
	EBGPMultiHop   bool
}

type advertisementConfig struct {
	IPFamily    ipfamily.Family
	Prefix      string
	Communities []string
	LocalPref   uint32
}

// routerName() defines the format of the key of the "Routers" map in the
// frrConfig struct.
func routerName(srcAddr string, myASN uint32) string {
	return fmt.Sprintf("%d@%s", myASN, srcAddr)
}

// neighborName() defines the format of key of the 'Neighbors' map in the
// routerConfig struct.
func neighborName(peerAddr string, ASN uint32) string {
	return fmt.Sprintf("%d@%s", ASN, peerAddr)
}

// templateConfig uses the template library to template
// 'globalConfigTemplate' using 'data'.
func templateConfig(data interface{}) (string, error) {
	i := 0
	currentCounterName := ""
	t, err := template.New("FRR Config Template").Funcs(
		template.FuncMap{
			"counter": func(counterName string) int {
				if currentCounterName != counterName {
					currentCounterName = counterName
					i = 0
				}
				i++
				return i
			},
			"frrIPFamily": func(ipFamily ipfamily.Family) string {
				if ipFamily == "ipv6" {
					return "ipv6"
				}
				return "ip"
			},
			"localPrefPrefixList": func(neighbor *neighborConfig, localPreference uint32) string {
				return fmt.Sprintf("%s-%d-%s-localpref-prefixes", neighbor.Addr, localPreference, neighbor.IPFamily)
			},
			"communityPrefixList": func(neighbor *neighborConfig, community string) string {
				return fmt.Sprintf("%s-%s-%s-community-prefixes", neighbor.Addr, community, neighbor.IPFamily)
			},
			"allowedPrefixList": func(neighbor *neighborConfig) string {
				return fmt.Sprintf("%s-pl-%s", neighbor.Addr, neighbor.IPFamily)
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
		}).Parse(configTemplate)
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
	return os.WriteFile(filename, []byte(config), 0644)
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
