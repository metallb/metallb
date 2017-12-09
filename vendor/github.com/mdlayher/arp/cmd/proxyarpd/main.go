package main

import (
	"bytes"
	"flag"
	"io"
	"log"
	"net"

	"github.com/mdlayher/arp"
	"github.com/mdlayher/ethernet"
)

var (
	// ifaceFlag is used to set a network interface for ARP traffic
	ifaceFlag = flag.String("i", "eth0", "network interface to use for ARP traffic")

	// ipFlag is used to set an IPv4 address to proxy ARP on behalf of
	ipFlag = flag.String("ip", "", "IP address for device to proxy ARP on behalf of")
)

func main() {
	flag.Parse()

	// Ensure valid interface and IPv4 address
	ifi, err := net.InterfaceByName(*ifaceFlag)
	if err != nil {
		log.Fatal(err)
	}
	ip := net.ParseIP(*ipFlag).To4()
	if ip == nil {
		log.Fatalf("invalid IPv4 address: %q", *ipFlag)
	}

	client, err := arp.Dial(ifi)
	if err != nil {
		log.Fatalf("couldn't create ARP client: %s", err)
	}

	// Handle ARP requests bound for designated IPv4 address, using proxy ARP
	// to indicate that the address belongs to this machine
	for {
		pkt, eth, err := client.Read()
		if err != nil {
			if err == io.EOF {
				log.Println("EOF")
				break
			}
			log.Fatalf("error processing ARP requests: %s", err)
		}

		// Ignore ARP replies
		if pkt.Operation != arp.OperationRequest {
			continue
		}

		// Ignore ARP requests which are not broadcast or bound directly for
		// this machine
		if !bytes.Equal(eth.Destination, ethernet.Broadcast) && !bytes.Equal(eth.Destination, ifi.HardwareAddr) {
			continue
		}

		log.Printf("request: who-has %s?  tell %s (%s)", pkt.TargetIP, pkt.SenderIP, pkt.SenderHardwareAddr)

		// Ignore ARP requests which do not indicate the target IP
		if !pkt.TargetIP.Equal(ip) {
			continue
		}

		log.Printf("  reply: %s is-at %s", ip, ifi.HardwareAddr)
		if err := client.Reply(pkt, ifi.HardwareAddr, ip); err != nil {
			log.Fatal(err)
		}
	}
}
