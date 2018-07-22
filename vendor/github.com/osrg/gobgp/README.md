# GoBGP: BGP implementation in Go

[![Build Status](https://travis-ci.org/osrg/gobgp.svg?branch=master)](https://travis-ci.org/osrg/gobgp/builds)
[![Slack Status](https://slackin-gobgp.mybluemix.net/badge.svg)](https://slackin-gobgp.mybluemix.net/)

GoBGP is an open source BGP implementation designed from scratch for
modern environment and implemented in a modern programming language,
[the Go Programming Language](http://golang.org/).

----

## To start using GoBGP

Try [a binary release](https://github.com/osrg/gobgp/releases/latest).

## To start developing GoBGP

You need a working [Go environment](https://golang.org/doc/install) (1.9 or newer).

```bash
$ go get -u github.com/golang/dep/cmd/dep
$ go get github.com/osrg/gobgp
$ cd $GOPATH/src/github.com/osrg/gobgp && dep ensure
```

## Documentation

### Using GoBGP

- [Getting Started](docs/sources/getting-started.md)
- CLI
  - [Typical operation examples](docs/sources/cli-operations.md)
  - [Complete syntax](docs/sources/cli-command-syntax.md)
- [Route Server](docs/sources/route-server.md)
- [Route Reflector](docs/sources/route-reflector.md)
- [Policy](docs/sources/policy.md)
- Zebra Integration
  - [FIB manipulation](docs/sources/zebra.md)
  - [Equal Cost Multipath Routing](docs/sources/zebra-multipath.md)
- [MRT](docs/sources/mrt.md)
- [BMP](docs/sources/bmp.md)
- [EVPN](docs/sources/evpn.md)
- [Flowspec](docs/sources/flowspec.md)
- [RPKI](docs/sources/rpki.md)
- [Managing GoBGP with your favorite language with gRPC](docs/sources/grpc-client.md)
- [Using GoBGP as a Go Native BGP library](docs/sources/lib.md)
- [Graceful Restart](docs/sources/graceful-restart.md)
- [Additional Paths](docs/sources/add-paths.md)
- [Peer Group](docs/sources/peer-group.md)
- [Dynamic Neighbor](docs/sources/dynamic-neighbor.md)
- [eBGP Multihop](docs/sources/ebgp-multihop.md)
- [TTL Security](docs/sources/ttl-security.md)
- Data Center Networking
  - [Unnumbered BGP](docs/sources/unnumbered-bgp.md)

### Externals

- [Tutorial: Using GoBGP as an IXP connecting router](http://www.slideshare.net/shusugimoto1986/tutorial-using-gobgp-as-an-ixp-connecting-router)

## Community, discussion and support

We have the [Slack](https://slackin-gobgp.mybluemix.net/) and [mailing
list](https://lists.sourceforge.net/lists/listinfo/gobgp-devel) for
questions, discussion, suggestions, etc.

You have code or documentation for GoBGP? Awesome! Send a pull
request. No CLA, board members, governance, or other mess.

## Licensing

GoBGP is licensed under the Apache License, Version 2.0. See
[LICENSE](https://github.com/osrg/gobgp/blob/master/LICENSE) for the full
license text.
