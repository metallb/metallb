#!/bin/bash

sudo modprobe br_netfilter
sudo sysctl net.bridge.bridge-nf-call-iptables=1

sudo apt -qq update
sudo apt -qq -y install ebtables ethtool apt-transport-https ca-certificates curl gnupg2 software-properties-common bridge-utils

curl -fsSL https://download.docker.com/linux/$(. /etc/os-release; echo "$ID")/gpg | sudo apt-key add -
curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo apt-key add -

sudo add-apt-repository \
   "deb [arch=amd64] https://download.docker.com/linux/$(. /etc/os-release; echo "$ID") \
   $(lsb_release -cs) \
   stable"
sudo add-apt-repository "deb http://apt.kubernetes.io/ kubernetes-xenial main"

sudo apt -qq update
sudo apt -qq -y install docker-ce kubelet kubeadm kubectl
