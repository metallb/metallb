package ndpcmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/mdlayher/ndp"
)

func sendReceiveLoop(ctx context.Context, c *ndp.Conn, ll *log.Logger, m ndp.Message, dst net.IP, check func(m ndp.Message) bool) error {
	for i := 0; ; i++ {
		msg, from, err := sendReceive(ctx, c, m, dst, check)
		switch err {
		case context.Canceled:
			fmt.Println()
			ll.Printf("canceled, sent %d message(s)", i+1)
			return err
		case errRetry:
			fmt.Print(".")
			continue
		case nil:
			fmt.Println()
			printMessage(ll, msg, from)
			return nil
		default:
			return err
		}
	}
}

func receiveLoop(ctx context.Context, c *ndp.Conn, ll *log.Logger) error {
	var recv int
	for {
		// Don't do any checks; return every message to be printed.
		msg, from, err := receive(ctx, c, nil)
		switch err {
		case context.Canceled:
			ll.Printf("received %d message(s)", recv)
			return nil
		case errRetry:
			continue
		case nil:
			recv++
			printMessage(ll, msg, from)
		default:
			return err
		}
	}
}

var errRetry = errors.New("retry")

func sendReceive(ctx context.Context, c *ndp.Conn, m ndp.Message, dst net.IP, check func(m ndp.Message) bool) (ndp.Message, net.IP, error) {
	if err := c.WriteTo(m, nil, dst); err != nil {
		return nil, nil, fmt.Errorf("failed to write message: %v", err)
	}

	return receive(ctx, c, check)
}

func receive(ctx context.Context, c *ndp.Conn, check func(m ndp.Message) bool) (ndp.Message, net.IP, error) {
	if err := c.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
		return nil, nil, fmt.Errorf("failed to set deadline: %v", err)
	}

	msg, _, from, err := c.ReadFrom()
	if err == nil {
		if check != nil && !check(msg) {
			// Read a message, but it isn't the one we want.  Keep trying.
			return nil, nil, errRetry
		}

		// Got a message that passed the check, if check was not nil.
		return msg, from, nil
	}

	// Was the context canceled already?
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	default:
	}

	// Was the error caused by a read timeout, and should the loop continue?
	if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
		return nil, nil, errRetry
	}

	return nil, nil, fmt.Errorf("failed to read message: %v", err)
}
