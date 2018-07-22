// Command ndp is a utility for working with the Neighbor Discovery Protocol.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"

	"github.com/mdlayher/ndp"
	"github.com/mdlayher/ndp/internal/ndpcmd"
)

func main() {
	var (
		ifiFlag    = flag.String("i", "", "network interface to use for NDP communication (default: automatic)")
		addrFlag   = flag.String("a", string(ndp.LinkLocal), "address to use for NDP communication (unspecified, linklocal, uniquelocal, global, or a literal IPv6 address)")
		targetFlag = flag.String("t", "", "IPv6 target address for neighbor solicitation NDP messages")
	)

	flag.Usage = func() {
		fmt.Println(usage)
		fmt.Println("Flags:")
		flag.PrintDefaults()
	}

	flag.Parse()
	ll := log.New(os.Stderr, "ndp> ", 0)

	ifi, err := findInterface(*ifiFlag)
	if err != nil {
		ll.Fatalf("failed to get interface: %v", err)
	}

	addr := ndp.Addr(*addrFlag)
	c, ip, err := ndp.Dial(ifi, addr)
	if err != nil {
		ll.Fatalf("failed to dial NDP connection: %v", err)
	}
	defer c.Close()

	var target net.IP
	if t := *targetFlag; t != "" {
		target = net.ParseIP(t)
		if target == nil {
			ll.Fatalf("failed to parse IPv6 address %q", t)
		}
	}

	sigC := make(chan os.Signal, 1)
	signal.Notify(sigC, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-sigC
		cancel()
	}()

	ll.Printf("interface: %s, link-layer address: %s, IPv6 address: %s",
		ifi.Name, ifi.HardwareAddr, ip)

	if err := ndpcmd.Run(ctx, c, ifi, flag.Arg(0), target); err != nil {
		// Context cancel means a signal was sent, so no need to log an error.
		if err == context.Canceled {
			os.Exit(1)
		}

		ll.Fatal(err)
	}
}

// findInterface attempts to find the specified interface.  If name is empty,
// it attempts to find a usable, up and ready, network interface.
func findInterface(name string) (*net.Interface, error) {
	if name != "" {
		ifi, err := net.InterfaceByName(name)
		if err != nil {
			return nil, fmt.Errorf("could not find interface %q: %v", name, err)
		}

		return ifi, nil
	}

	ifis, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, ifi := range ifis {
		// Is the interface up and not a loopback?
		if ifi.Flags&net.FlagUp != 1 || ifi.Flags&net.FlagLoopback != 0 {
			continue
		}

		// Does the interface have an IPv6 address assigned?
		addrs, err := ifi.Addrs()
		if err != nil {
			return nil, err
		}

		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}

			// Is this address an IPv6 address?
			if ipNet.IP.To16() != nil && ipNet.IP.To4() == nil {
				return &ifi, nil
			}
		}
	}

	return nil, errors.New("could not find a usable IPv6-enabled interface")
}

const usage = `ndp: utility for working with the Neighbor Discovery Protocol.

Examples:
  Listen for incoming NDP messages on interface eth0 to one of the interface's
  global unicast addresses.

    $ sudo ndp -i eth0 -a global listen
    $ sudo ndp -i eth0 -a 2001:db8::1 listen

  Send router solicitations on interface eth0 from the interface's link-local
  address until a router advertisement is received.

    $ sudo ndp -i eth0 -a linklocal rs

  Send neighbor solicitations on interface eth0 to a neighbor's link-local
  address until a neighbor advertisement is received.

    $ sudo ndp -i eth0 -a linklocal -t fe80::1 ns
`
