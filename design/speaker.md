# Speaker Design

MetalLB’s speaker is deployed as a container with a Dockerfile specified
[here](https://github.com/metallb/metallb/blob/main/speaker/Dockerfile) and
running the [“speaker” Go application](https://github.com/metallb/metallb/tree/main/speaker).
This "speaker" application is the main application that manages advertisement of
prefixes based on the creation of Kubernetes services. The Docker image and the
Go executable get built through [tasks.py](https://github.com/metallb/metallb/blob/main/tasks.py)
as part of the Metallb build process.

The "speaker" application supports a Layer 2 mode and a BGP mode. The Layer 2
mode advertises prefixes using Gratuitous ARP for IPv4 and Unsolicited Neighbor
Advertisement for IPv6. The BGP mode advertises using the BGP protocol.

## Components

The "speaker" application consists of a number of Go packages as illustrated
below:

```


┌───────────────┐
│               │
│    config     ├──┐
│ <<ConfigMap>> │  │                                         ┌───────────────┐
│               │  │reconcile                                │               │
└───────────────┘  │                                       ┌─┤  speakerlist  │
                   │                                       │ │  <<package>>  │
                   │                                       │ │               │
┌───────────────┐  │ ┌───────────────┐  ┌───────────────┐  │ └───────────────┘
│               │  │ │               │  │               │  │
│   services    ├──┼─┤      k8s      ├──┤     main      ├──┤
│  <<Service>>  │  │ │  <<package>>  │  │  <<package>>  │  │
│               │  │ │               │  │               │  │
└───────────────┘  │ └───────────────┘  └───────┬───────┘  │
                   │                    <<use>> │          │ ┌───────────────┐
                   │                            │          │ │               │
┌───────────────┐  │reconcile           ┌───────▼───────┐  │ │    config     │
│               │  │                    │               │  └─┤  <<package>>  │
│     nodes     │  │                    │   Protocol    │    │               │
│    <<Node>>   ├──┘                    │ <<interface>> │    └───────────────┘
│               │                       │               │
└───────────────┘                       └───────▲───────┘
                                                │<<implements>>
                                     ┌──────────┴────────────┐
                                     │                       │
                          ┌──────────┴────────┐     ┌────────┴──────┐
                          │       main::      │     │     main::    │
                          │ layer2_controller │     │ bgp_controller│
                          │     <<class>>     │     │   <<class>>   │
                          │                   │     │               │
                          └───────────────────┘     └───────┬───────┘
                                                            │
                                                            │  <<use>>
                                                    ┌───────▼───────┐
                                                    │               │
                                                    │    session    │
                                                    │ <<interface>> │
                                                    │               │
                                                    └───────▲───────┘
                                                            │ <<implements>>
                                                            │
                                                    ┌───────┴───────┐
                                                    │               │
                                                    │     bgp       │
                                                    │  <<package>>  │
                                                    │               │
                                                    └───────────────┘

```

### main

The main package contains the main() entrypoint for the speaker application. It
creates an Layer 2 and/or BGP controller, a speakerList and a k8s client. The
k8s client watches for changes to the MetalLB ConfigMap, service configuration,
and the node configuration and calls the appropriate protocol handler (e.g. L2,
BGP) via the “Protocol” Interface after ensuring the change is valid.

The BGP implementation of the “Protocol” interface can be found
[here](https://github.com/metallb/metallb/blob/main/speaker/bgp_controller.go).
This handles changes to the ConfigMap, service, and node configuration for BGP
pools. Based on these changes, BGP sessions are established/destroyed and
prefixes are advertised via the bgp package.

The Layer 2 implementation of the "Protocol" interface can be found
[here](https://github.com/metallb/metallb/blob/main/speaker/layer2_controller.go).

### bgp

The bgp package manages BGP sessions and advertisements. A BGP session is
created by the New() method and prefixes are advertised by the Set() method
which is part of the session interface.

BGP sessions are uniquely identified and configured by the parameters of the
New() method which are currently:

* **addr**: IP address and port of peer
* **srcAddr**: Source IP address
* **myASN**: Local ASN
* **routerID**: Specify the BGP Router ID of the session
* **asn**: ASN of the router that we are connecting to
* **holdTime**: BGP Hold Time specifies how long to maintain sessions for
* **Password**: TCP MD5 sock option for connection
* **myNode**

Advertisements are made for a session via the Set() method.

### speakerlist

speakerlist uses the Hashicorp memberlist library to create a cluster of
speakers, manage membership of this cluster and detect member failures using
a gossip protocol. This is only used in the speaker's Layer 2 mode.

### config

Parses a ConfigMap into an internal representation of the configuration
described by structs in the config package.

### k8s

A Kubernetes client/watcher.

## BGP session creation

New BGP sessions are established by calling newBGP() in the speaker's "main"
package which calls bgp.New() which in turn returns a session interface. The session
interface is the main interface used to advertise prefixes.
