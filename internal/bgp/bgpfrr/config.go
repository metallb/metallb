package bgpfrr

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"syscall"
	"text/template"
)

var configFileName = "/etc/frr_reloader/frr.conf"
var reloaderPidFileName = "/etc/frr_reloader/reloader.pid"

const configTemplate = `
log file /etc/frr/frr.log {{.Loglevel}}
log timestamp precision 3
hostname {{.Hostname}}

{{range .Routers -}}
router bgp {{.MyASN}} view {{.MyASN}}
  no bgp network import-check
  no bgp default ipv4-unicast
{{ if .RouterId }}
  bgp router-id {{.RouterId}}
{{- end }}
{{range .Neighbors }}
  neighbor {{.Addr}} remote-as {{.ASN}}
  {{ if .Port }}
  neighbor {{.Addr}} port {{.Port}}
  {{end}}
  {{ if .HoldTime }}
  neighbor {{.Addr}} timers 30 {{.HoldTime}}
  {{end}}
  {{ if .Password }}
  neighbor {{.Addr}} password {{.Password}}
  {{end}}
{{- end }}
{{range $n := .Neighbors -}}
{{range .Advertisements }}
  address-family {{.Version}} unicast
    neighbor {{$n.Addr}} activate
    network {{.Prefix}}
  exit-address-family
{{- end}}
{{end}}
{{end}}
`

type frrConfig struct {
	Loglevel string
	Hostname string
	Routers  map[string]*routerConfig
}

type routerConfig struct {
	MyASN     uint32
	RouterId  string
	Neighbors map[string]*neighborConfig
}

type neighborConfig struct {
	ASN            uint32
	Addr           string
	Port           uint16
	HoldTime       uint64
	Password       string
	Advertisements map[string]*advertisementConfig
}

type advertisementConfig struct {
	Version string
	Prefix  string
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
	t, err := template.New("FRR Config Template").Parse(configTemplate)
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
func reloadConfig() error {
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

func configFRR(config *frrConfig) error {
	filename, found := os.LookupEnv("FRR_CONFIG_FILE")
	if found {
		configFileName = filename
	}

	configString, err := templateConfig(config)
	if err != nil {
		return err
	}

	err = writeConfig(configString, configFileName)
	if err != nil {
		return err
	}

	err = reloadConfig()
	if err != nil {
		return err
	}

	return nil
}
