#!/bin/bash

CONTAINER_NAME=golint

set +e
sudo docker rm -f $CONTAINER_NAME
set -e
sudo docker run -dti --name $CONTAINER_NAME $1

cat > golint.sh << EOF
#!/bin/bash
set -e
export GOROOT=/usr/local/go
export GOPATH=\$HOME/go
export PATH=\$PATH:\$GOROOT/bin:\$GOPATH/bin

go get -u github.com/golang/lint/golint
#go get -u -insecure gitlab.cisco.com/ctao/vnf-agent/agent
#go get -u -insecure gitlab.cisco.com/ctao/vnf-agent/agent/cmd/vpp-agent

cd \$GOPATH/src/gitlab.cisco.com/ctao/vnf-agent
make golint 2>/opt/golint/golint_out.txt
EOF

chmod +x golint.sh
echo $CONTAINER_NAME
sudo docker exec $CONTAINER_NAME mkdir /opt/golint
sudo docker cp golint.sh $CONTAINER_NAME:/opt/golint/
sudo docker exec $CONTAINER_NAME ls -al /opt/golint/
sudo docker exec $CONTAINER_NAME /opt/golint/golint.sh
sudo docker cp $CONTAINER_NAME:/opt/golint/golint_out.txt .

sudo docker rm -f $CONTAINER_NAME

set +e
cat golint_out.txt |grep -v -e '^$' >results.txt
set -e
cat results.txt

if [ -s "results.txt" ] 
then
  exit 1
fi
