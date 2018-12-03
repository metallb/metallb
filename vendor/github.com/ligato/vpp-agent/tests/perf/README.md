# VPP Agent startup time analysis 

This is a draft version - tool for time analysis of VPP Agent startup.

## Getting Started

The tool assumes there is installled and running Kubernetes environment
Available scripts are to just there to do it. Bellow example explains
the meaning of output csv values.

### Installing

Steps to make it run:
- Run ./startK8.sh  to instal and init the Kubernetes environment
  to test it run: 
  kubectl get nodes
  you should get  output similar to:
  kubectl get nodes
  NAME                  STATUS    AGE       VERSION
  test-virtualbox       Ready     1d        v1.7.5

- Run ./download-images.sh  to download the docker images with preinstalled VPP Agent
  to test it run:
  docker images
  you should get the docker image named "ligato/vpp-agent" in the list of local repositiory
  
  REPOSITORY                                               TAG                 IMAGE ID            CREATED             SIZE
  ligato/vpp-agent                                         pantheon-dev        5ae90588c389        4 hours ago         444MB

  

## Running the tests

Once the Kubernetes is running and VPP Agent image is availalbe locally, you can run the script to analyse the times on VPP Agent startup.
- Run ./timemeasure.sh
  the script will start two pods, vnf-vpp and vswitch-vpp and loads the topology allowing network traffic.
  Then it collects the logs from vswitch-vpp pod .
  Then the pod is killed and after restart the logs are collected again.
  During test executuion the log files are stored under directory log/.
  The logs are proccessed and records the logged timestamps into the log/measuring_exp.csv file. This is in format that can be easily
  imported into the Excel for better visual analylis and statistics.
- Alternativelly you can specify number of the kills and restarts by number - as a parameter for ./timemeasure.sh . 
  Default value without parameter specified is 1.
  Example ./timemeasure.sh 30  - will kill the pod 30 times and collect the logs.
  Please see that  log/out.csv is just an intermediate result from which the times are calculated.
  The log/measuring_exp.csv is considered the real outputs. 
  Finally all the log files and csv files are zipped into the logresult.zip for further investigation and easy archiving
  

## Topology

There is possibility to load prepared topology into the etcd before the profiling is started
There are few scripts topology*.sh that do the loading and are called inside the timemeasure.sh script
It is easy to prepare your own topology according these scripts and modify function  setup() in timemeasure.sh
to load your own topology.

## Process to kill

The mechanism of restart the container is based on this approach:
supervisord is the main process, once it is killed, the container 
is getting restarted. Tehre is a eventlistener in supervisord conf file
defined. Its task is to listen on the vpp and vpp-agent processed.
If eighter of them is killed supervisod is killed to and container is restarted.
To be able to choose which process to kill, modify the timemeasure.sh script
variable kill_proc. If kill_proc=0 - VPP process is to be killed
if kill_proc=1  VPP-Agent is to be killed


## Vswitch.
There is also possibility to profile VPP acting as vswitch. For that purpose
a yaml file vswitch-vpp.yaml is available. To make it used in script find this two lines 
and use comments to choose proper yaml file .
Example for using vswitch yaml file
#kubectl apply -f vpp.yaml
kubectl apply -f vswitch-vpp.yaml
You must provide correct MAC ADDR in yaml file by editing the dev value.
    dpdk {
      dev 0000:00:08.0
      uio-driver igb_uio
    }
Value 0000:00:08.0 is just an example here.

### Output example

This is example of output that is in the measuring_exp.csv file.

#record,#run,step,timeline,relative time,relative to #record,duration(ms)
1,1,Measuring started,11:55:19.92454,00:00:00.0,0
2,1,Starting Agent,11:55:24.85929,00:00:4.93475,1
3,1,ETCD connected,11:55:24.86087,00:00:0.00158,2,0.5197990000
4,1,Kafka connected 11:55:24.85929,00:00:0.03132,2,2,
5,1,VPP connected,,-11:-55:-24.85929,2,
6,1,Resync done,11:55:26.90698,00:00:2.04769,2,9.9616360000
7,1,Agent ready,11:55:27.03256,00:00:2.17327,2
8,2,Container Killed,11:55:39.50288,19.57834,2
9,2,Starting Agent,11:55:43.61909,00:00:4.11621,8
10,2,ETCD connected,11:55:43.62086,00:00:0.00177,9,0.7439120000
11,2,Kafka connected,0,-11:-55:-43.61909,9,0
12,2,VPP connected,0,-11:-55:-43.61909,9,0
13,2,Resync done,11:55:44.39751,00:00:0.77842,9,3.1623150000
14,2,Agent ready,11:55:44.51965,00:00:0.90056,9
15,3,Container Killed,11:56:2.01177,22.50889,9

Explanation:
#record             - the incrementing value 
run                 -  shows which run the item belongs to
                    In the example above items 1-7 belong to first run                   
step                -  description of the event 
timeline            - shows the timestamps
relative time       - calculated value from two timestamps ( actual one and timestamp specified in next column
                    for example 6,1,Resync done,11:55:26.90698,00:00:2.04769,2,9.9616360000 on this line
                    value 00:00:2.04769 is calculated from actual timestamp (11:55:26.90698) minus timestamp on line 2 ( relative time #record is 2)
                    here it is 11:55:24.85929, result  is  11:55:26.90698 - 11:55:24.85929 =  00:00:2.04769
relative to #record - specifies #record which is relative time calculated to
duration(ms)        - some logs provide the internal measurement value

There are situations when the set of logs after the pod was killed is not available due to K8 issues
If it is a case, a record with ",Kill failed" is provided.

