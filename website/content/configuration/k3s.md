---
title: Issues with K3s
weight: 20
---

K3S come with its own service load balancer named Klipper. You need to disable it in order to run MetalLB.
To disable Klipper, run the server with the `--disable servicelb` option, as described in [K3s documentation](https://docs.k3s.io/networking/networking-services#disabling-servicelb)
)

