# System Integration

System integration is about exposing services or consuming services, either 
from local plugins/microservices or from external servers (including database, 
message bus, RPC calls).

Please follow:
## Timeouts
Timeouts are very important when implementing system integration.

```
TODO link to code of db or messaging that allows to configure global timeout
```

```
TODO link to code of db or messaging that allows to configure method level timeout using varargs mehotd(args, WithTimout())
```

## Reconnection
After a successful recovery from an HA event (restart, loss of connectivity),
a client MUST reconnect to a service it was consuming before the event and 
perform data resynchronization.  

```
TODO link to code/doc of db or messaging 
```

## AfterInit() failed to connect to a service
If a plugin implements a client that consumes an external service, and
the client is unable to connect to hte service before a timeout, the plugin
MUST propagate errors. The application will not start, and an external 
orchestrator must clean it up and start a new instance.. 

TODO assuming that there is a default deployment strategy for container-based 
cloud (as is with K8s) that will try to heal the container and basically 
recreates it.

```
TODO link to code of db or messaging plugin that propagates error 
```
