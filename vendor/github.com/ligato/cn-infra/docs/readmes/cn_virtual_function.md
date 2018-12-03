# Cloud-Native Virtual Network Functions (CNFs)
So what is a *cloud-native* virtual network function? 

A virtual network function (or VNF), as commonly known today, is a software
implementation of a network function that runs on one or more *virtual 
machines* (VMs) on top of the hardware networking infrastructure — routers,
switches, etc. Individual virtual network functions can be connected or
combined together as building blocks to offer a full-scale networking 
communication service. A VNF may be implemented as standalone entity using
existing networking and orchestration paradigms - for example being 
managed through CLI, SNMP or Netconf. Alternatively, an NFV may be a part
of an SDN architecture, where the control plane resides in an SDN 
controller and the data plane is implemented in the VNF.

A *cloud-native VNF* is a VNF designed for the emerging cloud environment -
it runs in a container rather than a VM, its lifecycle is orchestrated 
by a container orchestration system, such as Kubernetes, and it's using
cloud-native orchestration paradigms. In other words, its control/management
plane looks just like any other container based [12-factor app][1]. to 
orchestrator or external clients it exposes REST or gRPC APIs, data stored
in centralized KV data stores, communicate over message bus, cloud-friendly
logging and config, cloud friendly build & deployment process, etc.,
Depending on the desired functionality, scale and performance, a cloud-
native VNF may provide a high-performance data plane, such as the [VPP][2].

Cloud-native VNFs are also known as "CNFs".

[1]: https://12factor.net
[2]: https://fd.io
