#!/bin/bash

set -e

perl -pi -e 's/stretch/testing/g' /etc/apt/sources.list
apt -qq update
DEBIAN_FRONTEND=noninteractive apt -o Dpkg::Options::="--force-confold" -qq -y dist-upgrade
DEBIAN_FRONTEND=noninteractive apt -qq -y install libvirt-daemon-system virtinst virt-goodies netcat-openbsd xorriso libguestfs-tools
ln -s /usr/bin/xorrisofs /usr/bin/mkisofs
virsh pool-define-as --name=default --type=dir --target=/var/lib/libvirt/images
virsh pool-start default
virsh pool-autostart default
wget -O /var/lib/libvirt/images/fedora.qcow2 https://download.fedoraproject.org/pub/fedora/linux/releases/27/CloudImages/x86_64/images/Fedora-Cloud-Base-27-1.6.x86_64.qcow2
qemu-img resize /var/lib/libvirt/images/fedora.qcow2 10G
rm /dev/random
ln -s /dev/urandom /dev/random
