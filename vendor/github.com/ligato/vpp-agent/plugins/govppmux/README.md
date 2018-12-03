# GoVPP Mux

The `govppmux` is a Core Agent Plugin which allows other plugins to access VPP
independently on each other by means of connection multiplexing.

Any plugin (core or external) that interacts with VPP can ask `govppmux`
to get its own, potentially customized, communication channel to running VPP instance.
Behind the scenes, all channels share the same connection created during the plugin
initialization using govpp core.

## API

### Connection

By default, GoVPP connects to that instance of VPP which uses the default shared memory segment prefix. 
This is because it is assumed that there is only a single VPP running in a sand-boxed environment 
together with the agent (e.g. through containerization). In case VPP runs with customized SHM prefix,
or there are several VPP instances running, GoVPP needs to know the prefix in order to connect to the 
correct VPP instance - the prefix has to be put in the govppmux configuration file (govpp.conf) with 
key `shm-prefix` and value matching VPP shared memory prefix.

### Multiplexing

`NewAPIChannel` returns a new API channel for communication with VPP via govpp core.
It uses default buffer sizes for the request and reply Go channels (by default both are 100 messages long).

If it is expected that the VPP may get overloaded at peak loads, for example if the user plugin
sends configuration requests in bulks, then it is recommended to use `NewAPIChannelBuffered`
and increase the buffer size for requests appropriately. Similarly, `NewAPIChannelBuffered` allows
to configure the size of the buffer for responses. This is also useful since the buffer for responses
is also used to carry VPP notifications and statistics which may temporarily rapidly grow in size
and frequency. By increasing the reply channel size, the probability of dropping messages from VPP
decreases at the cost of increased memory footprint.

### Example 

The following example shows how to dump VPP interfaces using a multi-response request:
```
// Create a new VPP channel with the default configuration.
plugin.vppCh, err = govppmux.NewAPIChannel()
if err != nil {
    // Handle error condition...
}
// Close VPP channel.
defer safeclose.Close(plugin.vppCh)

req := &interfaces.SwInterfaceDump{}
reqCtx := plugin.vppCh.SendMultiRequest(req)

for {
    msg := &interfaces.SwInterfaceDetails{}
    stop, err := reqCtx.ReceiveReply(msg)
    if err != nil {
        // Handle error condition...
    }

    // Break out of the loop in case there are no more messages.
    if stop {
        break
    }

    log.Info("Found interface with sw_if_index=", msg.SwIfIndex)
    // Process the message...
}

```

## Configuration

The plugin allows to configure parameters of vpp health-check probe.
The items that can be configured are:
- *health check probe interval* - time between health check probes
- *health check reply timeout* - if the reply doesn't arrive until timeout
elapses probe is considered failed
- *health check threshold* - number of consequent failed health checks
until an error is reported
- *reply timeout* - if the reply from channel doesn't arrive until timeout
elapses, the request fails
- *shm-prefix* - used for connection to a VPP instance which is not using 
default shared memory prefix
- *resync-after-reconnect* - allows to run resync after recoonection

## Trace
 
Duration of the VPP binary api call can be measured using trace feature. These data are logged after 
every event(any resync, interfaces, bridge domains, fib entries etc.). Enable trace in govpp.conf: 
 
`trace-enabled: true` or  `trace-enabled: false`
  
Trace is disabled by default (if there is no config available). 