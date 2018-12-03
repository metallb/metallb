*** Settings ***
Documentation     Library which includes all other libs
...
...
...

Library     basic_operations.py
Resource    setup-teardown.robot
Resource    ssh.robot
Resource    docker.robot
Resource    configurations.robot
Resource    vat_term.robot
Resource    vpp_term.robot
Resource    lm_term.robot
Resource    vpp.robot
Resource    vpp_ctl.robot
Resource    rest_api.robot
Resource    vxlan.robot
Resource    linux.robot
# Resource    kubernetes/all_kube_libs.robot
Resource    SshCommons.robot
