#!/bin/bash

sudo kubeadm init --pod-network-cidr=10.250.0.0/16 --token $1
