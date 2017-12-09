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

You need a working [Go environment](https://golang.org/doc/install) (1.8 or newer).

```bash
$ go get -u github.com/golang/dep/cmd/dep
$ go get github.com/osrg/gobgp
$ cd $GOPATH/src/github.com/osrg/gobgp && dep ensure
```

## Documentation

### Using GoBGP
 * [Getting Started](https://github.com/osrg/gobgp/blob/master/docs/sources/getting-started.md)
 * CLI
   * [Typical operation examples](https://github.com/osrg/gobgp/blob/master/docs/sources/cli-operations.md)
   * [Complete syntax](https://github.com/osrg/gobgp/blob/master/docs/sources/cli-command-syntax.md)
 * [Route Server](https://github.com/osrg/gobgp/blob/master/docs/sources/route-server.md)
 * [Route Reflector](https://github.com/osrg/gobgp/blob/master/docs/sources/route-reflector.md)
 * [Policy](https://github.com/osrg/gobgp/blob/master/docs/sources/policy.md)
 * [FIB manipulation](https://github.com/osrg/gobgp/blob/master/docs/sources/zebra.md)
 * [MRT](https://github.com/osrg/gobgp/blob/master/docs/sources/mrt.md)
 * [BMP](https://github.com/osrg/gobgp/blob/master/docs/sources/bmp.md)
 * [EVPN](https://github.com/osrg/gobgp/blob/master/docs/sources/evpn.md)
 * [Flowspec](https://github.com/osrg/gobgp/blob/master/docs/sources/flowspec.md)
 * [RPKI](https://github.com/osrg/gobgp/blob/master/docs/sources/rpki.md)
 * [Managing GoBGP with your favorite language with GRPC](https://github.com/osrg/gobgp/blob/master/docs/sources/grpc-client.md)
 * [Using GoBGP as a Go Native BGP library](https://github.com/osrg/gobgp/blob/master/docs/sources/lib.md)
 * [Graceful Restart](https://github.com/osrg/gobgp/blob/master/docs/sources/graceful-restart.md)
 * Data Center Networking
   * [Unnumbered BGP](https://github.com/osrg/gobgp/blob/master/docs/sources/unnumbered-bgp.md)

### Externals
 * [Tutorial: Using GoBGP as an IXP connecting router](http://www.slideshare.net/shusugimoto1986/tutorial-using-gobgp-as-an-ixp-connecting-router)
 
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
