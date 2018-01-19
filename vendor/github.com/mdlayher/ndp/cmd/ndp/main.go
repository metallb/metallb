// Command ndp is a utility for working with the Neighbor Discovery Protocol.
package main

import (
	"context"
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
		ifiFlag  = flag.String("i", "eth0", "network interface to use for NDP communication")
		addrFlag = flag.String("a", string(ndp.LinkLocal), "address to use for NDP communication (unspecified, linklocal, uniquelocal, global, or a literal IPv6 address)")
	)

	flag.Usage = func() {
		fmt.Println(usage)
		fmt.Println("Flags:")
		flag.PrintDefaults()
	}

	flag.Parse()
	ll := log.New(os.Stderr, "ndp> ", 0)

	ifi, err := net.InterfaceByName(*ifiFlag)
	if err != nil {
		ll.Fatalf("failed to get interface %q: %v", *ifiFlag, err)
	}

	addr := ndp.Addr(*addrFlag)
	c, ip, err := ndp.Dial(ifi, addr)
	if err != nil {
		ll.Fatalf("failed to dial NDP connection: %v", err)
	}
	defer c.Close()

	sigC := make(chan os.Signal, 1)
	signal.Notify(sigC, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-sigC
		cancel()
	}()

	ll.Printf("interface: %s, link-layer address: %s, IPv6 address: %s",
		*ifiFlag, ifi.HardwareAddr, ip)

	if err := ndpcmd.Run(ctx, c, ifi, flag.Arg(0)); err != nil {
		// Context cancel means a signal was sent, so no need to log an error.
		if err == context.Canceled {
			os.Exit(1)
		}

		ll.Fatal(err)
	}
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
`
