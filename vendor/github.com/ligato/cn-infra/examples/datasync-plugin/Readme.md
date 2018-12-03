# DataSync example

### Requirements

To start the example, you have to have the ETCD running first.
Use the following command to pull the image and start the database:
```
sudo docker run -p 2379:2379 --name etcd --rm \
    quay.io/coreos/etcd:v3.0.16 /usr/local/bin/etcd \
    -advertise-client-urls http://0.0.0.0:2379 \
    -listen-client-urls http://0.0.0.0:2379
```

It will bring up the ETCD listening on port 2379 for the client communication.

### Usage

In the example, the location of the ETCD configuration file is defined
with the `-etcd-config` argument or through the `ETCD_CONFIG`
environment variable. By default, the application will try to search 
for `etcd.conf` in the current working directory. If the configuration 
file cannot be loaded or is not found, ETCD plugin tries to connect
using default configuration.

To run the example, type:
```
go run main.go deps.go [-etcd-config <config-filepath>]
```

