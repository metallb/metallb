// Command etherecho broadcasts a message to all machines in the same network
// segment, and listens for other messages from other etherecho servers.
//
// etherecho only works on Linux and BSD, and requires root permission or
// CAP_NET_ADMIN on Linux.
package main

import (
	"flag"
	"log"
	"net"
	"os"
	"time"

	"github.com/mdlayher/ethernet"
	"github.com/mdlayher/raw"
)

// Make use of an unassigned EtherType for etherecho.
// https://www.iana.org/assignments/ieee-802-numbers/ieee-802-numbers.xhtml
const etherType = 0xcccc

func main() {
	var (
		ifaceFlag = flag.String("i", "", "network interface to use to send and receive messages")
		msgFlag   = flag.String("m", "", "message to be sent (default: system's hostname)")
	)

	flag.Parse()

	// Open a raw socket on the specified interface, and configure it to accept
	// traffic with etherecho's EtherType.
	ifi, err := net.InterfaceByName(*ifaceFlag)
	if err != nil {
		log.Fatalf("failed to find interface %q: %v", *ifaceFlag, err)
	}

	c, err := raw.ListenPacket(ifi, etherType)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// Default message to system's hostname if empty.
	msg := *msgFlag
	if msg == "" {
		msg, err = os.Hostname()
		if err != nil {
			log.Fatalf("failed to retrieve hostname: %v", err)
		}
	}

	// Send messages in one goroutine, receive messages in another.
	go sendMessages(c, ifi.HardwareAddr, msg)
	go receiveMessages(c, ifi.MTU)

	// Block forever.
	select {}
}

// sendMessages continuously sends a message over a connection at regular intervals,
// sourced from specified hardware address.
func sendMessages(c net.PacketConn, source net.HardwareAddr, msg string) {
	// Message is broadcast to all machines in same network segment.
	f := &ethernet.Frame{
		Destination: ethernet.Broadcast,
		Source:      source,
		EtherType:   etherType,
		Payload:     []byte(msg),
	}

	b, err := f.MarshalBinary()
	if err != nil {
		log.Fatalf("failed to marshal ethernet frame: %v", err)
	}

	// Required by Linux, even though the Ethernet frame has a destination.
	// Unused by BSD.
	addr := &raw.Addr{
		HardwareAddr: ethernet.Broadcast,
	}

	// Send message forever.
	t := time.NewTicker(1 * time.Second)
	for range t.C {
		if _, err := c.WriteTo(b, addr); err != nil {
			log.Fatalf("failed to send message: %v", err)
		}
	}
}

// receiveMessages continuously receives messages over a connection. The messages
// may be up to the interface's MTU in size.
func receiveMessages(c net.PacketConn, mtu int) {
	var f ethernet.Frame
	b := make([]byte, mtu)

	// Keep receiving messages forever.
	for {
		n, addr, err := c.ReadFrom(b)
		if err != nil {
			log.Fatalf("failed to receive message: %v", err)
		}

		// Unpack Ethernet II frame into Go representation.
		if err := (&f).UnmarshalBinary(b[:n]); err != nil {
			log.Fatalf("failed to unmarshal ethernet frame: %v", err)
		}

		// Display source of message and message itself.
		log.Printf("[%s] %s", addr.String(), string(f.Payload))
	}
}
