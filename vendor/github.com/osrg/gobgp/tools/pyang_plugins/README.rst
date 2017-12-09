What's this ?
=============
This is a pyang plugin to generate config/bgp_configs.go from
openconfig yang files (see https://github.com/openconfig/public).

Prerequisites
=============
Please confirm $GOPATH is configured before the following steps.

How to use
==========
Set the environment variables for this tool::

   $ GOBGP_PATH=$GOPATH/src/github.com/osrg/gobgp/

Clone the required resources by using Git::

   $ cd $HOME
   $ git clone https://github.com/osrg/public
   $ git clone https://github.com/YangModels/yang
   $ git clone https://github.com/mbj4668/pyang

Setup environments for pyang::

   $ cd $HOME/pyang
   $ source ./env.sh

Generate config/bgp_configs.go from yang files::

   $ PYTHONPATH=. ./bin/pyang \
   --plugindir $GOBGP_PATH/tools/pyang_plugins \
   -p $HOME/yang/standard/ietf/RFC \
   -p $HOME/public/release/models \
   -p $HOME/public/release/models/bgp \
   -p $HOME/public/release/models/policy \
   -f golang \
   $HOME/public/release/models/bgp/openconfig-bgp.yang \
   $HOME/public/release/models/policy/openconfig-routing-policy.yang \
   $GOBGP_PATH/tools/pyang_plugins/gobgp.yang \
   | gofmt > $GOBGP_PATH/config/bgp_configs.go
