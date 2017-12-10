#!/bin/sh

NR_PEERS=8
BRIDGE_NAME=br0
CONFIG_DIR=/usr/local/gobgp
GOBGP_DOCKER_NAME=gobgp
USE_HOST=0

check_user() {
    if [ `whoami` = "root" ]; then
        echo "Super user cannot execute! Please execute as non super user"
        exit 2
    fi
}

run_quagga() {
    local docker_name=q$1
    docker run --privileged=true -v $CONFIG_DIR/$docker_name:/etc/quagga --name $docker_name -id osrg/quagga
    sudo pipework $BRIDGE_NAME $docker_name 10.0.0.$1/16
}

stop_quagga() {
    local docker_name=q$1
    docker rm -f $docker_name
}

delete_bridge() {
    local name=$1
    local sysfs_name=/sys/class/net/$name
    if [ -e $sysfs_name ]; then
        sudo ifconfig $name down
        sudo brctl delbr $name
    fi
}

while getopts c:n:u OPT
do
    case $OPT in
	c) CONFIG_DIR="$OPTARG"
	    ;;
	n) NR_PEERS="$OPTARG"
	    ;;
	u) USE_HOST=1
	    ;;
	*) echo "Unknown option"
	    exit 1
	    ;;
    esac
done

shift $((OPTIND - 1))

case "$1" in
    start)
	i=1
	while [ $i -le $NR_PEERS ]
	do
	    run_quagga $i
	    i=$(( i+1 ))
	done
	if [ $USE_HOST -eq 1 ]; then
	    sudo ip addr add 10.0.255.1/16 dev $BRIDGE_NAME
	else
	    docker run --privileged=true -v $CONFIG_DIR:/mnt -d --name $GOBGP_DOCKER_NAME -id osrg/gobgp
	    sudo pipework $BRIDGE_NAME $GOBGP_DOCKER_NAME 10.0.255.1/16
	fi
	;;
    stop)
	i=1
	while [ $i -le $NR_PEERS ]
	do
	    stop_quagga $i
	    i=$(( i+1 ))
	done
	delete_bridge $BRIDGE_NAME
	docker rm -f $GOBGP_DOCKER_NAME
	;;
    install)
	sudo apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys 36A1D7869245C8950F966E92D8576A8BA88D21E9
	sudo sh -c "echo deb https://get.docker.io/ubuntu docker main > /etc/apt/sources.list.d/docker.list"
	sudo apt-get update
	sudo apt-get install -y --force-yes lxc-docker-1.3.2
	sudo ln -sf /usr/bin/docker.io /usr/local/bin/docker
	sudo gpasswd -a `whoami` docker
        sudo apt-get install -y --force-yes emacs23-nox
        sudo apt-get install -y --force-yes wireshark
	sudo apt-get install -y --force-yes iputils-arping
        sudo apt-get install -y --force-yes bridge-utils
        sudo apt-get install -y --force-yes tcpdump
        sudo apt-get install -y --force-yes lv
	sudo wget https://raw.github.com/jpetazzo/pipework/master/pipework -O /usr/local/bin/pipework
	sudo chmod 755 /usr/local/bin/pipework
        sudo docker pull osrg/quagga
        sudo docker pull osrg/gobgp
	sudo mkdir /usr/local/gobgp
	sudo docker run --privileged=true -v /usr/local/gobgp:/mnt --name gobgp --rm osrg/gobgp go run /root/gobgp/tools/route-server/quagga-rsconfig.go -c /mnt
	;;
    *)
	echo $1
	echo "Usage: root-server-docker {start|stop|install}"
	exit 2
	;;
esac


