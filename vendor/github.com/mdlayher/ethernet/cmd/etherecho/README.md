etherecho
=========

Command `etherecho` broadcasts a message to all machines in the same network
segment, and listens for other messages from other `etherecho` servers.

`etherecho` only works on Linux and BSD, and requires root permission or
`CAP_NET_ADMIN` on Linux.

Usage
-----

```
$ etherecho -h
Usage of etherecho:
  -i string
        network interface to use to send and receive messages
  -m string
        message to be sent (default: system's hostname)
```

Example
-------

Start an instance of `etherecho` on two machines on the same network segment:

```
foo $ etherecho -i eth0
```
```
bar $ etherecho -i eth0
```

Both machines should begin seeing messages from each other at regular intervals:

```
foo $ etherecho -i eth0
2017/06/14 00:03:13 [aa:aa:aa:aa:aa:aa] bar
2017/06/14 00:03:14 [aa:aa:aa:aa:aa:aa] bar
2017/06/14 00:03:15 [aa:aa:aa:aa:aa:aa] bar
```
```
bar $ etherecho -i eth0
2017/06/14 00:03:13 [bb:bb:bb:bb:bb:bb] foo
2017/06/14 00:03:14 [bb:bb:bb:bb:bb:bb] foo
2017/06/14 00:03:15 [bb:bb:bb:bb:bb:bb] foo
```

Additional machines can be added, so long as they reside on the same network
segment.