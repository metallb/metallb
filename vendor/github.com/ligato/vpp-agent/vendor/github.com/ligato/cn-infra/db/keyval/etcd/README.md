# Etcd plugin

The Etcd plugin provides access to an etcd key-value data store.

## Configuration

- Location of the Etcd configuration file can be defined either by the 
  command line flag `etcd-config` or set via the `ETCD_CONFIG`
  environment variable.

## Status Check

- If injected, Etcd plugin will use StatusCheck plugin to periodically
  issue a minimalistic GET request to check for the status
  of the connection.
  The etcd connection state affects the global status of the agent.
  If agent cannot establish connection with etcd, both the readiness
  and the liveness probe from the [probe plugin](../../../health/probe)
  will return a negative result (accessible only via REST API in such
  case).

## Compacting

You can compact Etcd using two ways.

- using API by calling `plugin.Compact()` which will compact the database
  to the current revision.
- using config file by setting `auto-compact` option to the duration of
  period that you want the Etcd to be compacted.

## Reconnect resynchronization

- If connection to the ETCD is interrupted, resync can be automatically called 
  after re-connection. This option is disabled by default and has to be allowed
  in the `etcd.conf` file.
  
  Set `resync-after-reconnect` to `true` to enable the feature.
