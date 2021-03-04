module go.universe.tf/metallb

go 1.12

require (
	github.com/armon/go-radix v1.0.0 // indirect
	github.com/dgryski/go-farm v0.0.0-20190423205320-6a90982ecee2 // indirect
	github.com/eapache/channels v1.1.0 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/go-kit/kit v0.9.0
	github.com/golang/protobuf v1.4.3
	github.com/google/go-cmp v0.5.2
	github.com/hashicorp/memberlist v0.1.7
	github.com/mdlayher/arp v0.0.0-20190313224443-98a83c8a2717
	github.com/mdlayher/ethernet v0.0.0-20190606142754-0394541c37b7
	github.com/mdlayher/ndp v0.0.0-20190419144644-012988d57f9a
	github.com/mikioh/ipaddr v0.0.0-20190404000644-d465c8ab6721
	github.com/osrg/gobgp v2.0.0+incompatible
	github.com/prometheus/client_golang v1.0.0
	github.com/satori/go.uuid v1.2.0 // indirect
	github.com/spf13/viper v1.7.0 // indirect
	github.com/vishvananda/netlink v1.0.0 // indirect
	github.com/vishvananda/netns v0.0.0-20190625233234-7109fa855b0f // indirect
	golang.org/x/sys v0.0.0-20210124154548-22da62e12c0c
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	k8s.io/klog v0.3.1
)

// Force using right version of client-go, as 'go get -u' will pull old one.
replace k8s.io/client-go => k8s.io/client-go v0.20.2
