arpc
=====

Command `arpc` provides a simple ARP client which can be used to retrieve
hardware addresses of other machines in a LAN using their IPv4 addresses.

Usage
-----

```
$ ./arpc -h
Usage of ./arpc:
  -d=1s: timeout for ARP request
  -i="eth0": network interface to use for ARP request
  -ip="": IPv4 address destination for ARP request
```

Request hardware address for IPv4 address:

```
$ ./arpc -i eth0 -ip 192.168.1.1
192.168.1.1 -> 00:12:7f:eb:6b:40
```
