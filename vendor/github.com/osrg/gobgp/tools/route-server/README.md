Route Server testing env
========================

Preparation
-----------
Set up Ubuntu 14.04 Server Edition Virtual Machine environment. We
tested this with Fusion on Mac OS X and VirtualBox on Windows 8.

Setup
-----
Open a terminal and execute the following commands:

```
% sudo apt-get install -y --force-yes wget
% wget https://raw.githubusercontent.com/osrg/gobgp/master/tools/route-server/route-server-docker.sh
% chmod +x route-server-docker.sh
% ./route-server-docker.sh install
```

All necessary software will be installed. You can find all configuration files (for Quagga and gobgp) at /usr/local/gobgp. Let's make sure:

```
fujita@ubuntu:~$ find /usr/local/gobgp|sort
/usr/local/gobgp
/usr/local/gobgp/gobgpd.conf
/usr/local/gobgp/q1
/usr/local/gobgp/q1/bgpd.conf
/usr/local/gobgp/q2
/usr/local/gobgp/q2/bgpd.conf
/usr/local/gobgp/q3
/usr/local/gobgp/q3/bgpd.conf
/usr/local/gobgp/q4
/usr/local/gobgp/q4/bgpd.conf
/usr/local/gobgp/q5
/usr/local/gobgp/q5/bgpd.conf
/usr/local/gobgp/q6
/usr/local/gobgp/q6/bgpd.conf
/usr/local/gobgp/q7
/usr/local/gobgp/q7/bgpd.conf
/usr/local/gobgp/q8
/usr/local/gobgp/q8/bgpd.conf
```

Before going to playing with this environment, you need to log out and log in again.

Start
-----
```
% ./route-server-docker.sh start
```
Eight containers for Quagga and one container for gobgp started.

Now ready to start gobgp so let's go into the container for gobgp:

```
% docker exec -it gobgp bash
```

You are supposed to get a console like the following:

```
root@ec36881de971:~#
```

```
root@e7cb66415e2f:~# go run gobgp/bgpd.go -f /mnt/gobgpd.conf
INFO[0000] Peer 10.0.0.1 is added
INFO[0000] Peer 10.0.0.2 is added
INFO[0000] Peer 10.0.0.3 is added
INFO[0000] Peer 10.0.0.4 is added
INFO[0000] Peer 10.0.0.5 is added
INFO[0000] Peer 10.0.0.6 is added
INFO[0000] Peer 10.0.0.7 is added
INFO[0000] Peer 10.0.0.8 is added       
```

After some time, you'll see more messages on the console (it means
Quagga bgpd connected to gobgp).

On a different console, let's check the status of Quagga.

```
% docker exec -it q1 bash
```

You can do something like:

```
root@a40ac8058ca7:/# telnet localhost bgpd
```

btw, the password is "zebra".

The names of Quagga containers are q1, q2, q3, q4, q5, q6, q7, and q8
respectively. You can execute bash on any container with 'docker exec'
command.

Stop
----
The following command stops and cleans up everything.

```
% ./route-server-docker.sh stop
```

You can safely excute this command whenever something goes wrong and start again.
