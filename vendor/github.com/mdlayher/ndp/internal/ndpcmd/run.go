// Package ndpcmd provides the commands for the ndp utility.
package ndpcmd

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/mdlayher/ndp"
)

// Run runs the ndp utility.
func Run(ctx context.Context, c *ndp.Conn, ifi *net.Interface, op string, target net.IP) error {
	switch op {
	// listen is the default when no op is specified..
	case "listen", "":
		return listen(ctx, c)
	case "ns":
		return sendNS(ctx, c, ifi.HardwareAddr, target)
	case "rs":
		return sendRS(ctx, c, ifi.HardwareAddr)
	default:
		return fmt.Errorf("unrecognized operation: %q", op)
	}
}

func listen(ctx context.Context, c *ndp.Conn) error {
	ll := log.New(os.Stderr, "ndp listen> ", 0)
	ll.Println("listening for messages")

	// Also listen for router solicitations from other hosts, even though we
	// will never reply to them.
	if err := c.JoinGroup(net.IPv6linklocalallrouters); err != nil {
		return err
	}

	if err := receiveLoop(ctx, c, ll); err != nil {
		return fmt.Errorf("failed to read message: %v", err)
	}

	return nil
}

func sendNS(ctx context.Context, c *ndp.Conn, addr net.HardwareAddr, target net.IP) error {
	ll := log.New(os.Stderr, "ndp ns> ", 0)

	ll.Printf("neighbor solicitation:\n    - source link-layer address: %s", addr.String())

	// Always multicast the message to the target's solicited-node multicast
	// group as if we have no knowledge of its MAC address.
	snm, err := ndp.SolicitedNodeMulticast(target)
	if err != nil {
		return fmt.Errorf("failed to determine solicited-node multicast address: %v", err)
	}

	m := &ndp.NeighborSolicitation{
		TargetAddress: target,
		Options: []ndp.Option{
			&ndp.LinkLayerAddress{
				Direction: ndp.Source,
				Addr:      addr,
			},
		},
	}

	// Expect neighbor advertisement messages with the correct target address.
	check := func(m ndp.Message) bool {
		na, ok := m.(*ndp.NeighborAdvertisement)
		if !ok {
			return false
		}

		return na.TargetAddress.Equal(target)
	}

	if err := sendReceiveLoop(ctx, c, ll, m, snm, check); err != nil {
		if err == context.Canceled {
			return err
		}

		return fmt.Errorf("failed to send neighbor solicitation: %v", err)
	}

	return nil
}

func sendRS(ctx context.Context, c *ndp.Conn, addr net.HardwareAddr) error {
	ll := log.New(os.Stderr, "ndp rs> ", 0)

	ll.Printf("router solicitation:\n    - source link-layer address: %s", addr.String())

	m := &ndp.RouterSolicitation{
		Options: []ndp.Option{
			&ndp.LinkLayerAddress{
				Direction: ndp.Source,
				Addr:      addr,
			},
		},
	}

	// Expect any router advertisement message.
	check := func(m ndp.Message) bool {
		_, ok := m.(*ndp.RouterAdvertisement)
		return ok
	}

	if err := sendReceiveLoop(ctx, c, ll, m, net.IPv6linklocalallrouters, check); err != nil {
		if err == context.Canceled {
			return err
		}

		return fmt.Errorf("failed to send router solicitation: %v", err)
	}

	return nil
}
