// SPDX-License-Identifier:Apache-2.0

package metrics

type metric struct {
	Name string
	Help string
}

var (
	Namespace = "metallb"
	Subsystem = "bgp"

	SessionUp = metric{
		Name: "session_up",
		Help: "BGP session state (1 is up, 0 is down)",
	}

	UpdatesSent = metric{
		Name: "updates_total",
		Help: "Number of BGP UPDATE messages sent",
	}

	Prefixes = metric{
		Name: "announced_prefixes_total",
		Help: "Number of prefixes currently being advertised on the BGP session",
	}
)
