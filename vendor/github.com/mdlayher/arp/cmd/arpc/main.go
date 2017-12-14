// Command arpc provides a simple ARP client which can be used to retrieve
// hardware addresses of other machines in a LAN using their IPv4 address.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/mdlayher/arp"
)

var (
	// durFlag is used to set a timeout for an ARP request
	durFlag = flag.Duration("d", 1*time.Second, "timeout for ARP request")

	// ifaceFlag is used to set a network interface for ARP requests
	ifaceFlag = flag.String("i", "eth0", "network interface to use for ARP request")

	// ipFlag is used to set an IPv4 address destination for an ARP request
	ipFlag = flag.String("ip", "", "IPv4 address destination for ARP request")
)

func main() {
	flag.Parse()

	// Ensure valid network interface
	ifi, err := net.InterfaceByName(*ifaceFlag)
	if err != nil {
		log.Fatal(err)
	}

	// Set up ARP client with socket
	c, err := arp.Dial(ifi)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	// Set request deadline from flag
	if err := c.SetDeadline(time.Now().Add(*durFlag)); err != nil {
		log.Fatal(err)
	}

	// Request hardware address for IP address
	ip := net.ParseIP(*ipFlag).To4()
	mac, err := c.Resolve(ip)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s -> %s", ip, mac)
}
