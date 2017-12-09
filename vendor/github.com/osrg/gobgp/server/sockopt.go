// Copyright (C) 2016 Nippon Telegraph and Telephone Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// +build !linux,!openbsd

package server

import (
	"fmt"
	"net"

	log "github.com/sirupsen/logrus"
)

func SetTcpMD5SigSockopt(l *net.TCPListener, address string, key string) error {
	return setTcpMD5SigSockopt(l, address, key)
}

func SetListenTcpTTLSockopt(l *net.TCPListener, ttl int) error {
	return setListenTcpTTLSockopt(l, ttl)
}

func SetTcpTTLSockopt(conn *net.TCPConn, ttl int) error {
	return setTcpTTLSockopt(conn, ttl)
}

func SetTcpMinTTLSockopt(conn *net.TCPConn, ttl int) error {
	return setTcpMinTTLSockopt(conn, ttl)
}

type TCPDialer struct {
	net.Dialer

	// MD5 authentication password.
	AuthPassword string

	// The TTL value to set outgoing connection.
	Ttl uint8

	// The minimum TTL value for incoming packets.
	TtlMin uint8
}

func (d *TCPDialer) DialTCP(addr string, port int) (*net.TCPConn, error) {
	if d.AuthPassword != "" {
		log.WithFields(log.Fields{
			"Topic": "Peer",
			"Key":   addr,
		}).Warn("setting md5 for active connection is not supported")
	}
	if d.Ttl != 0 {
		log.WithFields(log.Fields{
			"Topic": "Peer",
			"Key":   addr,
		}).Warn("setting ttl for active connection is not supported")
	}
	if d.TtlMin != 0 {
		log.WithFields(log.Fields{
			"Topic": "Peer",
			"Key":   addr,
		}).Warn("setting min ttl for active connection is not supported")
	}

	raddr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(addr, fmt.Sprintf("%d", port)))
	if err != nil {
		return nil, fmt.Errorf("invalid remote address: %s", err)
	}
	laddr, err := net.ResolveTCPAddr("tcp", d.LocalAddr.String())
	if err != nil {
		return nil, fmt.Errorf("invalid local address: %s", err)
	}

	dialer := net.Dialer{LocalAddr: laddr, Timeout: d.Timeout}
	conn, err := dialer.Dial("tcp", raddr.String())
	if err != nil {
		return nil, err
	}
	return conn.(*net.TCPConn), nil
}
