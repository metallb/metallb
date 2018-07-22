package ndpcmd

import (
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/mdlayher/ndp"
)

func printMessage(ll *log.Logger, m ndp.Message, from net.IP) {
	switch m := m.(type) {
	case *ndp.NeighborAdvertisement:
		printNA(ll, m, from)
	case *ndp.NeighborSolicitation:
		printNS(ll, m, from)
	case *ndp.RouterAdvertisement:
		printRA(ll, m, from)
	case *ndp.RouterSolicitation:
		printRS(ll, m, from)
	default:
		ll.Printf("%s %#v", from, m)
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

func printRS(ll *log.Logger, rs *ndp.RouterSolicitation, from net.IP) {
	var opts string
	for _, o := range rs.Options {
		opts += fmt.Sprintf("        - %s\n", optStr(o))
	}

	ll.Printf(
		rsFormat,
		from.String(),
		opts,
	)
}

const rsFormat = `router solicitation from %s:
    - options:
%s`

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

func printNS(ll *log.Logger, ns *ndp.NeighborSolicitation, from net.IP) {
	var opts string
	for _, o := range ns.Options {
		opts += fmt.Sprintf("        - %s\n", optStr(o))
	}

	ll.Printf(
		nsFormat,
		from.String(),
		ns.TargetAddress.String(),
		opts,
	)
}

const nsFormat = `neighbor solicitation from %s:
    - target address: %s
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
	case *ndp.RecursiveDNSServer:
		var ss []string
		for _, s := range o.Servers {
			ss = append(ss, s.String())
		}
		servers := strings.Join(ss, ", ")

		return fmt.Sprintf("recursive DNS servers: lifetime: %s, servers: %s", o.Lifetime, servers)
	default:
		panic(fmt.Sprintf("unrecognized option: %v", o))
	}
}
