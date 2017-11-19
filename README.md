# Metallb

Metallb is a load-balancer implementation for bare metal Kubernetes
clusters, using BGP.

This is not an official Google project.

## Known weirdness

MetalLB's BGP implementation is slightly stricter than the spec when
it comes to CIDR prefixes. BGP encodes a prefix as a CIDR mask size
and an IP address. Trailing bytes of the IP that are fully masked out
are elided, but a partially masked byte is included in its
entirety. RFC 4271 states "Note that the value of trailing bits is
irrelevant."

This makes parsing and encoding of UPDATE messages non-idempotent,
because there are multiple valid encodings for the same prefix.

For ease of fuzz testing, MetalLB's implementation rejects prefixes in
UPDATE messages if they are not in "normal form", with the masked bits
set to zero.

If you encounter a BGP implementation that fails to interoperate with
MetalLB because of this constraint, please file a bug.
