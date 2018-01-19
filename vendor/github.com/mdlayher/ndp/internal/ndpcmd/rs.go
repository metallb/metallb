package ndpcmd

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/mdlayher/ndp"
)

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

	for i := 0; ; i++ {
		if err := c.WriteTo(m, nil, net.IPv6linklocalallrouters); err != nil {
			return fmt.Errorf("failed to write router solicitation: %v", err)
		}

		ra, from, err := receiveRA(c)
		if err == nil {
			fmt.Println()
			printRA(ll, ra, from)
			return nil
		}

		// Was the context canceled already?
		select {
		case <-ctx.Done():
			fmt.Println()
			ll.Printf("sent %d router solicitation(s)", i+1)
			return ctx.Err()
		default:
		}

		// Was the error caused by a read timeout, and should the loop continue?
		if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			fmt.Print(".")
			continue
		}

		return fmt.Errorf("failed to read router advertisement: %v", err)
	}
}

func receiveRA(c *ndp.Conn) (*ndp.RouterAdvertisement, net.IP, error) {
	if err := c.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
		return nil, nil, err
	}

	for {
		msg, _, from, err := c.ReadFrom()
		if err != nil {
			return nil, nil, err
		}

		ra, ok := msg.(*ndp.RouterAdvertisement)
		if !ok {
			continue
		}

		return ra, from, nil
	}
}

func printRA(ll *log.Logger, ra *ndp.RouterAdvertisement, from net.IP) {
	var opts string
	for _, o := range ra.Options {
		opts += fmt.Sprintf("        - %s\n", optStr(o))
	}

	var flags string
	if ra.ManagedConfiguration {
		flags += "M"
	}
	if ra.OtherConfiguration {
		flags += "O"
	}
	if ra.MobileIPv6HomeAgent {
		flags += "H"
	}
	if ra.NeighborDiscoveryProxy {
		flags += "P"
	}

	ll.Printf(
		raFormat,
		from.String(),
		ra.CurrentHopLimit,
		flags,
		ra.RouterSelectionPreference,
		ra.RouterLifetime,
		ra.ReachableTime,
		ra.RetransmitTimer,
		opts,
	)
}

const raFormat = `router advertisement from: %s:
    - hop limit:        %d
    - flags:            [%s]
    - preference:       %d
    - router lifetime:  %s
    - reachable time:   %s
    - retransmit timer: %s
    - options:
%s`

func optStr(o ndp.Option) string {
	switch o := o.(type) {
	case *ndp.LinkLayerAddress:
		dir := "source"
		if o.Direction == ndp.Target {
			dir = "target"
		}

		return fmt.Sprintf("%s link-layer address: %s", dir, o.Addr.String())
	case *ndp.MTU:
		return fmt.Sprintf("MTU: %d", *o)
	case *ndp.PrefixInformation:
		var flags string
		if o.OnLink {
			flags += "O"
		}
		if o.AutonomousAddressConfiguration {
			flags += "A"
		}

		return fmt.Sprintf("prefix information: %s/%d, flags: [%s], valid: %s, preferred: %s",
			o.Prefix.String(),
			o.PrefixLength,
			flags,
			o.ValidLifetime,
			o.PreferredLifetime,
		)
	case *ndp.RawOption:
		return fmt.Sprintf("type: %03d, value: %v", o.Type, o.Value)
	default:
		panic(fmt.Sprintf("unrecognized option: %v", o))
	}
}

func printNA(ll *log.Logger, na *ndp.NeighborAdvertisement, from net.IP) {
	var opts string
	for _, o := range na.Options {
		opts += fmt.Sprintf("        - %s\n", optStr(o))
	}

	ll.Printf(
		naFormat,
		from.String(),
		na.Router,
		na.Solicited,
		na.Override,
		na.TargetAddress.String(),
		opts,
	)
}

const naFormat = `neighbor advertisement from %s:
    - router:         %t
    - solicited:      %t
    - override:       %t
    - target address: %s
    - options:
%s`
