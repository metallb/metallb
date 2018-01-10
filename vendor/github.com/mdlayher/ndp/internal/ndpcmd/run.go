// Package ndpcmd provides the commands for the ndp utility.
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

// Run runs the ndp utility.
func Run(ctx context.Context, c *ndp.Conn, ifi *net.Interface, op string) error {
	switch op {
	case "listen":
		return listen(ctx, c)
	case "rs":
		return sendRS(ctx, c, ifi.HardwareAddr)
	default:
		return fmt.Errorf("unrecognized operation: %q", op)
	}
}

func listen(ctx context.Context, c *ndp.Conn) error {
	ll := log.New(os.Stderr, "ndp listen> ", 0)
	ll.Println("listening for messages")

	var recv int
	for {
		if err := c.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
			return err
		}

		m, _, from, err := c.ReadFrom()
		if err == nil {
			recv++
			printMessage(ll, m, from)
			continue
		}

		// Was the context canceled already?
		select {
		case <-ctx.Done():
			ll.Printf("received %d message(s)", recv)
			return ctx.Err()
		default:
		}

		// Was the error caused by a read timeout, and should the loop continue?
		if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			continue
		}

		return fmt.Errorf("failed to read message: %v", err)
	}
}

func printMessage(ll *log.Logger, m ndp.Message, from net.IP) {
	switch m := m.(type) {
	case *ndp.NeighborAdvertisement:
		printNA(ll, m, from)
	case *ndp.RouterAdvertisement:
		printRA(ll, m, from)
	default:
		ll.Printf("%s %#v", from, m)
	}
}
