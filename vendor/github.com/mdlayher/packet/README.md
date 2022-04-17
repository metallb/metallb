# packet [![Test Status](https://github.com/mdlayher/packet/workflows/Test/badge.svg)](https://github.com/mdlayher/packet/actions) [![Go Reference](https://pkg.go.dev/badge/github.com/mdlayher/packet.svg)](https://pkg.go.dev/github.com/mdlayher/packet)  [![Go Report Card](https://goreportcard.com/badge/github.com/mdlayher/packet)](https://goreportcard.com/report/github.com/mdlayher/packet)

Package `packet` provides access to Linux packet sockets (`AF_PACKET`). MIT
Licensed.

## Stability

See the [CHANGELOG](./CHANGELOG.md) file for a description of changes between
releases.

In order to reduce the maintenance burden, this package is only supported on
Go 1.12+. Older versions of Go lack critical features and APIs which are
necessary for this package to function correctly.

**If you depend on this package in your applications, please use Go modules.**

## History

One of my first major Go networking projects was
[`github.com/mdlayher/raw`](https://github.com/mdlayher/raw), which provided
access to Linux `AF_PACKET` sockets and *BSD equivalent mechanisms for sending
and receiving Ethernet frames. However, the *BSD support languished and I lack
the expertise and time to properly maintain code for operating systems I do not
use on a daily basis.

Package `packet` is a successor to package `raw`, but exclusively focused on
Linux and `AF_PACKET` sockets. The APIs are nearly identical, but with a few
changes which take into account some of the lessons learned while working on
`raw`.

Users are highly encouraged to migrate any existing Linux uses of `raw` to
package `packet` instead. This package will be supported for the foreseeable
future and will receive continued updates as necessary.
