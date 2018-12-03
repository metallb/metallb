# Consul plugin

The Consul plugin provides access to a consul key-value data store.

## Configuration

- Location of the Consul configuration file can be defined either by the
  command line flag `consul-config` or set via the `CONSUL_CONFIG`
  environment variable.

## Status Check

- If injected, Consul plugin will use StatusCheck plugin to periodically
  issue a minimalistic GET request to check for the status of the connection.
  The consul connection state affects the global status of the agent.
  If agent cannot establish connection with consul, both the readiness
  and the liveness probe from the [probe plugin](../../../health/probe)
  will return a negative result (accessible only via REST API in such
  case).

## Reconnect resynchronization

- If connection to the Consul is interrupted, resync can be automatically called
  after re-connection. This option is disabled by default and has to be allowed
  in the etcd.conf file.

  Set `resync-after-reconnect` to `true` to enable the feature.
