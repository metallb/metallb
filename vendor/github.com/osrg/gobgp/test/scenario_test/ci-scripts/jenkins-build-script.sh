#!/usr/bin/env sh

# renew GOPATH
rm -rf /usr/local/jenkins/{bin,pkg,src}
mkdir /usr/local/jenkins/{bin,pkg,src}
mkdir -p /usr/local/jenkins/src/github.com/osrg/

export GOBGP_IMAGE=gobgp
export GOPATH=/usr/local/jenkins
export GOROOT=/usr/local/go
export GOBGP=/usr/local/jenkins/src/github.com/osrg/gobgp
export WS=`pwd`

# clear docker.log
if [ "${BUILD_TAG}" != "" ]; then
    sudo sh -c ": > /var/log/upstart/docker.log"
fi

rm -f ${WS}/nosetest*.xml
cp -r ../workspace $GOBGP
pwd
cd $GOBGP
ls -al
git log | head -20

sudo docker rmi $(sudo docker images | grep "^<none>" | awk '{print $3}')
sudo docker rm -f $(sudo docker ps -a -q)

for link in $(ip li | awk '/(_br|veth)/{sub(":","", $2); print $2}')
do
    sudo ip li set down $link
    sudo ip li del $link
done

sudo docker rmi $GOBGP_IMAGE
sudo fab -f $GOBGP/test/lib/base.py make_gobgp_ctn:tag=$GOBGP_IMAGE
[ "$?" != 0 ] && exit "$?"

cd $GOBGP/gobgpd
$GOROOT/bin/go get -v

cd $GOBGP/test/scenario_test
./run_all_tests.sh

if [ "${BUILD_TAG}" != "" ]; then
    cd ${WS}
    mkdir jenkins-log-${BUILD_NUMBER}
    sudo cp *.xml jenkins-log-${BUILD_NUMBER}/
    sudo cp /var/log/upstart/docker.log jenkins-log-${BUILD_NUMBER}/docker.log
    sudo chown -R jenkins:jenkins jenkins-log-${BUILD_NUMBER}

    tar cvzf jenkins-log-${BUILD_NUMBER}.tar.gz jenkins-log-${BUILD_NUMBER}
    s3cmd put jenkins-log-${BUILD_NUMBER}.tar.gz s3://gobgp/jenkins/
    rm -rf jenkins-log-${BUILD_NUMBER} jenkins-log-${BUILD_NUMBER}.tar.gz
fi

