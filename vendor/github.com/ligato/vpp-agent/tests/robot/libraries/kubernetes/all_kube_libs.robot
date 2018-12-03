*** Settings ***
Documentation    Library which imports all Kubernetes-related libraries.
...
...
...

Library     kube_parser.py
Library     kube_config_gen.py

Resource    KubeAdm.robot
Resource    KubeCtl.robot
Resource    KubeEnv.robot
Resource    KubeSetup.robot
Resource    Restarts_Setup.robot