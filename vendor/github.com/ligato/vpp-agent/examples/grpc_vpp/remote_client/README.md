# GRPC remote client example

The example uses vpp-agent as a GRPC server. The configuration uses GRPC service to call remote 
procedure in the agent to create desired configuration. The example shows how to create data change
and resync request.

How to run example:
1. Run vpp-agent with GRPC server enabled - start it with the grpc configuration file with `endpoint`
defined.

```
vpp-agent --grpc-config=/opt/vpp-agent/dev/grpc.conf
```

2. Run GRPC client (example):
```
go run main.go
```

Two flags can be set:
* `-address=<address>` - for grpc server address/socket-file (otherwise localhost will be used)
* `-socket-type=<type>` - options are tcp, tcp4, tcp6, unix or unixpacket. Defaults to tcp if not set

The example creates resync request with configuration which is then updated with data change request.
