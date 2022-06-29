// SPDX-License-Identifier:Apache-2.0

package frr

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"syscall"
	"text/template"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
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
{{- range $n := .Neighbors }}
route-map {{$n.Addr}}-in deny 20
{{- range $a := .Advertisements }}
{{- if not (eq $a.LocalPref 0)}}
{{frrIPFamily $a.IPFamily}} prefix-list {{localPrefPrefixList $n $a.LocalPref}} permit {{$a.Prefix}}
route-map {{$n.Addr}}-out permit {{counter $n.Addr}}
  match {{frrIPFamily $a.IPFamily}} address prefix-list {{localPrefPrefixList $n $a.LocalPref}}
  set local-preference {{$a.LocalPref}}
  on-match next
{{- end }}
{{- range $c := $a.Communities }}
{{frrIPFamily $a.IPFamily}} prefix-list {{communityPrefixList $n $c}} permit {{$a.Prefix}}
route-map {{$n.Addr}}-out permit {{counter $n.Addr}}
  match {{frrIPFamily $a.IPFamily}} address prefix-list {{communityPrefixList $n $c}}
  set community {{$c}} additive
  on-match next
{{- end }}
{{frrIPFamily $a.IPFamily}} prefix-list {{allowedPrefixList $n}} permit {{$a.Prefix}}
{{- end }}
route-map {{$n.Addr}}-out permit {{counter $n.Addr}}
  match ip address prefix-list {{allowedPrefixList $n}}
route-map {{$n.Addr}}-out permit {{counter $n.Addr}}
  match ipv6 address prefix-list {{allowedPrefixList $n}}
ip prefix-list {{allowedPrefixList $n}} deny any
ipv6 prefix-list {{allowedPrefixList $n}} deny any
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
{{range .Neighbors }}
  neighbor {{.Addr}} remote-as {{.ASN}}
  {{- if .EBGPMultiHop }}
  neighbor {{.Addr}} ebgp-multihop
  {{- end }}
  {{ if .Port -}}
  neighbor {{.Addr}} port {{.Port}}
  {{- end }}
  neighbor {{.Addr}} timers {{.KeepaliveTime}} {{.HoldTime}}
  {{ if .Password -}}
  neighbor {{.Addr}} password {{.Password}}
  {{- end }}
  {{ if .SrcAddr -}}
  neighbor {{.Addr}} update-source {{.SrcAddr}}
  {{- end }}
{{- if ne .BFDProfile ""}} 
  neighbor {{.Addr}} bfd profile {{.BFDProfile}}
{{- end }}
{{- if  mustDisableConnectedCheck .IPFamily $r.MyASN .ASN .EBGPMultiHop }}
  neighbor {{.Addr}} disable-connected-check
{{- end }}
{{- end }}
{{range $n := .Neighbors -}}
{{/* no bgp default ipv4-unicast prevents peering if no address families are defined. We declare an ipv4 one for the peer to make the pairing happen */}}
{{- if eq (len .Advertisements) 0}}
  address-family ipv4 unicast
    neighbor {{$n.Addr}} activate
    neighbor {{$n.Addr}} route-map {{$n.Addr}}-in in
	neighbor {{$n.Addr}} route-map {{$n.Addr}}-out out
  exit-address-family
  address-family ipv6 unicast
    neighbor {{$n.Addr}} activate
    neighbor {{$n.Addr}} route-map {{$n.Addr}}-in in
	neighbor {{$n.Addr}} route-map {{$n.Addr}}-out out
  exit-address-family
{{- end}}
{{- range .Advertisements }}
  address-family {{.IPFamily.String}} unicast
    neighbor {{$n.Addr}} activate
    neighbor {{$n.Addr}} route-map {{$n.Addr}}-in in
    network {{.Prefix}}
    neighbor {{$n.Addr}} route-map {{$n.Addr}}-out out
  exit-address-family
{{- end}}
{{end -}}
{{end }}
{{- if gt (len .BFDProfiles) 0}}
bfd
{{- range .BFDProfiles }}
  profile {{.Name}}
    {{ if .ReceiveInterval -}}
    receive-interval {{.ReceiveInterval}}
    {{end -}}
    {{ if .TransmitInterval -}}
    transmit-interval {{.TransmitInterval}}
    {{end -}}
    {{ if .DetectMultiplier -}}
    detect-multiplier {{.DetectMultiplier}}
    {{end -}}
    {{ if .EchoMode -}}
    echo-mode
    {{end -}}
    {{ if .EchoInterval -}}
    echo-interval {{.EchoInterval}}
    {{end -}}
    {{ if .PassiveMode -}}
    passive-mode
    {{end -}}
    {{ if .MinimumTTL -}}
    minimum-ttl {{ .MinimumTTL }}
    {{end -}}
{{ end }}
{{ end }}`

type frrConfig struct {
	Loglevel    string
	Hostname    string
	Routers     map[string]*routerConfig
	BFDProfiles []BFDProfile
}

// TODO: having global prefix lists works only because we advertise all the addresses
// to all the neighbors. Once this constraint is changed, we may need prefix-lists per neighbor.

type routerConfig struct {
	MyASN     uint32
	RouterId  string
	Neighbors map[string]*neighborConfig
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
	return ioutil.WriteFile(filename, []byte(config), 0644)
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
	reload <-chan *frrConfig,
	reloadInterval time.Duration,
	failureRetryInterval time.Duration) {
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
				config = newCfg
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
