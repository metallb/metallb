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
// +build linux

package server

import (
	"fmt"
	"net"
	"os"
	"syscall"
	"unsafe"
)

const (
	tcpMD5SIG       = 14 // TCP MD5 Signature (RFC2385)
	ipv6MinHopCount = 73 // Generalized TTL Security Mechanism (RFC5082)
)

type tcpmd5sig struct {
	ss_family uint16
	ss        [126]byte
	// padding the struct
	_      uint16
	keylen uint16
	// padding the struct
	_   uint32
	key [80]byte
}

func buildTcpMD5Sig(address string, key string) (tcpmd5sig, error) {
	t := tcpmd5sig{}
	addr := net.ParseIP(address)
	if addr.To4() != nil {
		t.ss_family = syscall.AF_INET
		copy(t.ss[2:], addr.To4())
	} else {
		t.ss_family = syscall.AF_INET6
		copy(t.ss[6:], addr.To16())
	}

	t.keylen = uint16(len(key))
	copy(t.key[0:], []byte(key))

	return t, nil
}

func setTCPMD5SigSockopt(l *net.TCPListener, address string, key string) error {
	t, err := buildTcpMD5Sig(address, key)
	if err != nil {
		return err
	}
	b := *(*[unsafe.Sizeof(t)]byte)(unsafe.Pointer(&t))

	sc, err := l.SyscallConn()
	if err != nil {
		return err
	}
	return setsockOptString(sc, syscall.IPPROTO_TCP, tcpMD5SIG, string(b[:]))
}

func setListenTCPTTLSockopt(l *net.TCPListener, ttl int) error {
	family := extractFamilyFromTCPListener(l)
	sc, err := l.SyscallConn()
	if err != nil {
		return err
	}
	return setsockoptIpTtl(sc, family, ttl)
}

func setTCPTTLSockopt(conn *net.TCPConn, ttl int) error {
	family := extractFamilyFromTCPConn(conn)
	sc, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	return setsockoptIpTtl(sc, family, ttl)
}

func setTCPMinTTLSockopt(conn *net.TCPConn, ttl int) error {
	family := extractFamilyFromTCPConn(conn)
	sc, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	level := syscall.IPPROTO_IP
	name := syscall.IP_MINTTL
	if family == syscall.AF_INET6 {
		level = syscall.IPPROTO_IPV6
		name = ipv6MinHopCount
	}
	return setsockOptInt(sc, level, name, ttl)
}

func setsockoptTcpMD5Sig(fd int, address string, key string) error {
	t, err := buildTcpMD5Sig(address, key)
	if err != nil {
		return err
	}
	b := *(*[unsafe.Sizeof(t)]byte)(unsafe.Pointer(&t))
	return os.NewSyscallError("setsockopt", syscall.SetsockoptString(fd, syscall.IPPROTO_TCP, tcpMD5SIG, string(b[:])))
}

func setsockoptIpTtl2(fd int, family int, value int) error {
	level := syscall.IPPROTO_IP
	name := syscall.IP_TTL
	if family == syscall.AF_INET6 {
		level = syscall.IPPROTO_IPV6
		name = syscall.IPV6_UNICAST_HOPS
	}
	return os.NewSyscallError("setsockopt", syscall.SetsockoptInt(fd, level, name, value))
}

func setsockoptIpMinTtl(fd int, family int, value int) error {
	level := syscall.IPPROTO_IP
	name := syscall.IP_MINTTL
	if family == syscall.AF_INET6 {
		level = syscall.IPPROTO_IPV6
		name = ipv6MinHopCount
	}
	return os.NewSyscallError("setsockopt", syscall.SetsockoptInt(fd, level, name, value))
}

type tcpDialer struct {
	net.Dialer

	// MD5 authentication password.
	AuthPassword string

	// The TTL value to set outgoing connection.
	TTL uint8

	// The minimum TTL value for incoming packets.
	TTLMin uint8
}

func (d *tcpDialer) DialTCP(addr string, port int) (*net.TCPConn, error) {
	var family int
	var ra, la syscall.Sockaddr

	raddr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(addr, fmt.Sprintf("%d", port)))
	if err != nil {
		return nil, fmt.Errorf("invalid remote address: %s", err)
	}
	laddr, err := net.ResolveTCPAddr("tcp", d.LocalAddr.String())
	if err != nil {
		return nil, fmt.Errorf("invalid local address: %s", err)
	}
	if raddr.IP.To4() != nil {
		family = syscall.AF_INET
		rsockaddr := &syscall.SockaddrInet4{Port: port}
		copy(rsockaddr.Addr[:], raddr.IP.To4())
		ra = rsockaddr
		lsockaddr := &syscall.SockaddrInet4{}
		copy(lsockaddr.Addr[:], laddr.IP.To4())
		la = lsockaddr
	} else {
		family = syscall.AF_INET6
		rsockaddr := &syscall.SockaddrInet6{Port: port}
		copy(rsockaddr.Addr[:], raddr.IP.To16())
		ra = rsockaddr
		var zone uint32
		if laddr.Zone != "" {
			if intf, err := net.InterfaceByName(laddr.Zone); err != nil {
				return nil, err
			} else {
				zone = uint32(intf.Index)
			}
		}
		lsockaddr := &syscall.SockaddrInet6{ZoneId: zone}
		copy(lsockaddr.Addr[:], laddr.IP.To16())
		la = lsockaddr
	}

	sockType := syscall.SOCK_STREAM | syscall.SOCK_CLOEXEC | syscall.SOCK_NONBLOCK
	proto := 0
	fd, err := syscall.Socket(family, sockType, proto)
	if err != nil {
		return nil, err
	}
	fi := os.NewFile(uintptr(fd), "")
	defer fi.Close()
	// A new socket was created so we must close it before this
	// function returns either on failure or success. On success,
	// net.FileConn() in newTCPConn() increases the refcount of
	// the socket so this fi.Close() doesn't destroy the socket.
	// The caller must call Close() with the file later.
	// Note that the above os.NewFile() doesn't play with the
	// refcount.

	if err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1); err != nil {
		return nil, os.NewSyscallError("setsockopt", err)
	}

	if err = syscall.SetsockoptInt(fd, syscall.IPPROTO_TCP, syscall.TCP_NODELAY, 1); err != nil {
		return nil, os.NewSyscallError("setsockopt", err)
	}

	if d.AuthPassword != "" {
		if err = setsockoptTcpMD5Sig(fd, addr, d.AuthPassword); err != nil {
			return nil, err
		}
	}

	if d.TTL != 0 {
		if err = setsockoptIpTtl2(fd, family, int(d.TTL)); err != nil {
			return nil, err
		}
	}

	if d.TTLMin != 0 {
		if err = setsockoptIpMinTtl(fd, family, int(d.TTL)); err != nil {
			return nil, err
		}
	}

	if err = syscall.Bind(fd, la); err != nil {
		return nil, os.NewSyscallError("bind", err)
	}

	newTCPConn := func(fi *os.File) (*net.TCPConn, error) {
		if conn, err := net.FileConn(fi); err != nil {
			return nil, err
		} else {
			return conn.(*net.TCPConn), err
		}
	}

	err = syscall.Connect(fd, ra)
	switch err {
	case syscall.EINPROGRESS, syscall.EALREADY, syscall.EINTR:
		// do timeout handling
	case nil, syscall.EISCONN:
		return newTCPConn(fi)
	default:
		return nil, os.NewSyscallError("connect", err)
	}

	epfd, e := syscall.EpollCreate1(syscall.EPOLL_CLOEXEC)
	if e != nil {
		return nil, e
	}
	defer syscall.Close(epfd)

	var event syscall.EpollEvent
	events := make([]syscall.EpollEvent, 1)

	event.Events = syscall.EPOLLIN | syscall.EPOLLOUT | syscall.EPOLLPRI
	event.Fd = int32(fd)
	if e = syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, fd, &event); e != nil {
		return nil, e
	}

	for {
		nevents, e := syscall.EpollWait(epfd, events, int(d.Timeout/1000000) /*msec*/)
		if e != nil {
			return nil, e
		}
		if nevents == 0 {
			return nil, fmt.Errorf("timeout")
		} else if nevents == 1 && events[0].Fd == int32(fd) {
			nerr, err := syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_ERROR)
			if err != nil {
				return nil, os.NewSyscallError("getsockopt", err)
			}
			switch err := syscall.Errno(nerr); err {
			case syscall.EINPROGRESS, syscall.EALREADY, syscall.EINTR:
			case syscall.Errno(0), syscall.EISCONN:
				return newTCPConn(fi)
			default:
				return nil, os.NewSyscallError("getsockopt", err)
			}
		} else {
			return nil, fmt.Errorf("unexpected epoll behavior")
		}
	}
}
