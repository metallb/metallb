// SPDX-License-Identifier:Apache-2.0

package udp

import (
	"errors"
	"net"
	"strings"
	"time"
)

// Check checks the if the agnhost server set up at the given address / port
// replies to the echo command.
func Check(address string) error {
	s, err := net.ResolveUDPAddr("udp4", address)
	if err != nil {
		return err
	}
	c, err := net.DialUDP("udp4", nil, s)
	if err != nil {
		return err
	}
	_, err = c.Write([]byte("echo test"))
	if err != nil {
		return err
	}
	buffer := make([]byte, 1024)
	err = c.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err != nil {
		return err
	}
	_, err = c.Read(buffer)
	if err != nil {
		return err
	}
	if !strings.Contains(string(buffer), "test") {
		return errors.New("UDP echo test failed|" + string(buffer) + "|")
	}
	return nil
}
