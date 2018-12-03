#!/bin/bash

set +e
sudo docker rmi -f dev_cn_infra_shrink 
sudo docker rm -f shrink
set -e

sudo docker run -itd --name shrink dev_cn_infra bash
sudo docker export shrink >shrink.tar
sudo docker rm -f shrink
sudo docker import -c "WORKDIR /root/" -c 'CMD ["/usr/bin/supervisord", "-c", "/etc/supervisord.conf"]' shrink.tar dev_cn_infra_shrink
rm shrink.tar
