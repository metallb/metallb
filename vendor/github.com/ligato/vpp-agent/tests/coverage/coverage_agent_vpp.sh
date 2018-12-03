#!/bin/bash

CONTAINER_NAME=coverage_agent_vpp

set +e
sudo docker rm -f $CONTAINER_NAME
set -e
sudo docker run -dti --name $CONTAINER_NAME $1

cat > coverage.sh << EOF
#!/bin/bash
set -e
export GOROOT=/usr/local/go
export GOPATH=\$HOME/go
export PATH=\$PATH:\$GOROOT/bin:\$GOPATH/bin

go get github.com/axw/gocov/...
go get github.com/AlekSi/gocov-xml

#go get -u -insecure gitlab.cisco.com/ctao/vnf-agent/agent
#go get -u -insecure gitlab.cisco.com/ctao/vnf-agent/agent/cmd/vpp-agent
cd \$GOPATH/src/gitlab.cisco.com/ctao/vnf-agent
#cd agent/flavors/vpp
#gocov test -tags=integration ./... | gocov-xml > /opt/coverage/coverage.xml
make test-cover-xml
EOF

chmod +x coverage.sh
echo $CONTAINER_NAME
sudo docker exec $CONTAINER_NAME mkdir /opt/coverage
sudo docker cp coverage.sh $CONTAINER_NAME:/opt/coverage/
sudo docker exec $CONTAINER_NAME ls -al /opt/coverage/
sudo docker exec $CONTAINER_NAME /opt/coverage/coverage.sh
sudo docker cp $CONTAINER_NAME:/tmp/coverage.xml .

sudo docker rm -f $CONTAINER_NAME
