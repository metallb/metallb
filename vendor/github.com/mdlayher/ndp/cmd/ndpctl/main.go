package main

import (
	"log"
	"net"
	"time"

	"github.com/mdlayher/ndp"
)

func main() {
	ifi, err := net.InterfaceByName("eth0")
	if err != nil {
		log.Fatalf("failed to get interface: %v", err)
	}

	c, llAddr, err := ndp.Dial(ifi)
	if err != nil {
		log.Fatalf("failed to dial NDP: %v", err)
	}
	defer c.Close()

	go func() {
		for {
			m, cm, src, err := c.ReadFrom()
			if err != nil {
				log.Printf("read: %v", err)
				continue
			}

			log.Println("read:", m, cm, src)
		}
	}()

	m := &ndp.NeighborAdvertisement{
		TargetAddress: llAddr,
		Options: []ndp.Option{
			&ndp.LinkLayerAddress{
				Direction: ndp.Target,
				Addr:      ifi.HardwareAddr,
			},
		},
	}

	for i := 0; i < 10; i++ {
		log.Printf("send: %+v", m)

		if err := c.WriteTo(m, nil, net.IPv6linklocalallnodes); err != nil {
			log.Fatalf("failed to write: %v", err)
		}

		time.Sleep(500 * time.Millisecond)
	}
}
