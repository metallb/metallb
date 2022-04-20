# CHANGELOG

## Unreleased

## v1.0.0

- Initial stable commit! The API is mostly a direct translation of the previous
  `github.com/mdlayher/raw` package APIs, with some updates to make everything
  focused explicitly on Linux and `AF_PACKET` sockets. Functionally, the two
  packages are equivalent, and `*raw.Conn` is now backed by `*packet.Conn` in
  the latest version of the `raw` package.
