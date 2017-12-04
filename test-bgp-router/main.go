package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"

	"github.com/golang/glog"
)

func main() {
	if err := installNatRule(); err != nil {
		glog.Exitf("Failed to install NAT rule: %s", err)
	}
	if err := writeBirdConfig(); err != nil {
		glog.Exitf("Failed to write bird config: %s", err)
	}
	if err := runTCPDump(); err != nil {
		glog.Exitf("Failed to start tcpdump: %s", err)
	}
	if err := runBird(); err != nil {
		glog.Exitf("Trying to start Bird: %s", err)
	}

	http.HandleFunc("/", status)
	http.HandleFunc("/pcap", writePcap)
	http.ListenAndServe(":8080", nil)
}

func nodeIP() string {
	return os.Getenv("METALLB_NODE_IP")
}

func runBird() error {
	if err := os.Mkdir("/run/bird", 0600); err != nil {
		return err
	}
	c := exec.Command("/usr/sbin/bird", "-d", "-c", "/etc/bird/bird.conf")
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Start(); err != nil {
		return err
	}
	go func() {
		if err := c.Wait(); err != nil {
			glog.Exitf("Bird exited with an error: %s", err)
		}
		glog.Exitf("Bird exited")
	}()
	return nil
}

func runTCPDump() error {
	if err := os.Mkdir("/run/tcpdump", 0600); err != nil {
		return err
	}
	c := exec.Command("/usr/sbin/tcpdump", "-i", "eth0", "-w", "/run/tcpdump/pcap", "tcp", "port", "1179")
	if err := c.Start(); err != nil {
		return err
	}
	go func() {
		if err := c.Wait(); err != nil {
			glog.Exitf("tcpdump exited with an error: %s", err)
		}
		glog.Exitf("tcpdump exited")
	}()
	return nil
}

func writeBirdConfig() error {
	cfg := fmt.Sprintf(`
router id 10.0.0.100;
listen bgp port 1179;
log stderr all;
debug protocols all;
protocol device {
}
protocol static {
  route %s/32 via "eth0";
}
protocol bgp minikube {
  local 10.0.0.100 as 64512;
  neighbor %s as 64512;
  passive;
  error wait time 1, 2;
}
`, nodeIP(), nodeIP())
	if err := ioutil.WriteFile("/etc/bird/bird.conf", []byte(cfg), 0644); err != nil {
		return err
	}
	return nil
}

func installNatRule() error {
	c := exec.Command("/sbin/iptables", "-t", "nat", "-A", "INPUT", "-p", "tcp", "--dport", "1179", "-j", "SNAT", "--to", os.Getenv("METALLB_NODE_IP"))
	return c.Run()
}
