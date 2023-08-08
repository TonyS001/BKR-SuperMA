# BKR-SuperMA

## preparation

We need to prepare a working instance on AWS, and another 4-100 test instances to run nodes and clients.
To quickly create AWS instances, you can refer to [narwhal repository](https://github.com/asonnino/narwhal/tree/master/benchmark).

There are 4 executables.  
- `create` is the programme that creates the config files.
- `BKRSuperMA` is the SMR server running the consensus algorithm.
- `client` is the client programme that issues requests to a server and receives responses and measures latency.
- `coor` is the programme that notifies the client to start test and then gather statistics from clients.

You can build from source or use the binary we provide.
Please put them in correct paths. 

### build from source

To build the 4 executables:
```
# under BKR-SuperMA/
go build -o BKRSuperMA
cd client
go build -o client
cd ../coordinator
go build -o coor
cd ../test/script
go build -o create
```

### directly use our binary

```
# under BKR-SuperMA/
cp binary/BKRSuperMA .
cp binary/client client/
cp binary/coor coordinator/
cp binary/create test/script/
```

## test

### deploy to remote nodes

Use `test/script/aws.py` to get information of cloud instances on AWS.

 Note that you need to modify  `aws.py` before you run it.
 - Specify regions your instances are located corresponding to your applied AWS instances.
 - Add your AWS `access_key` and `secret_key` in the corresponding fields:
```
# we used for 10 and 100 nodes
# regions = ["us-east-2","ap-southeast-1","ap-northeast-1","ca-central-1","eu-central-1"]

# for 4 nodes
regions = ["us-east-2","ap-southeast-1","ap-northeast-1","eu-central-1"]

# for 4 nodes with 1 far away
# regions = ["ap-east-1","ap-southeast-1","ap-northeast-1","eu-central-1"]

access_key = "[your access_key]"
secret_key = "[your secret_key]"
```

 Then:
```
# under BKR-SuperMA/
mkdir -p conf/multi
mkdir -p log
cd test/script
python3 aws.py [instance name] [dev name] [running num]
```
 `instance name` is the name of instances where your nodes and clients are running (remote).
 And `dev name` is the name of your current working instance, on which `coordinator` will run.

 The result of the above commands are three config files: `test/script/nodes.json`, `test/script/devip.json` and `conf/nodes.txt`.

Then use `test/script/create` to generate other config files, and `test/script/deliverAll.sh` to deliver the binary and config files to remote test instances:
```
# under BKR-SuperMA/test/script/
./create -n [system node num] -f [tolerable fault num] -p [running num] -b [byzantine num]
./deliverAll.sh [running num]
```

`system node num` is $n$  and `tolerable fault num` is $f$ in paper.
`running num` is the number of instances that actually run in the test.
E.g. in the crash faults test, if `running num = n - f`, it means `f` nodes are not running.  

### prepare three terminals
Now open three terminals to run `node`, `client` and `coordinator` respectively. They should be started in the following order.

### run node

Use `test/script/nodes.sh` to start `node` firstly.

```
# under BKR-SuperMA/test/script/
./nodes.sh [running num] [batch size] [payload] [test time] [payload channels] [proposing node num] [test mode]
```

`batch size` is the maximum number of requests that can be included in a proposal. `payload` (Bytes) is the size of a request. `test time` (second) is the duration of a test. `payload channel` is the number of channels used to transfer payload among `node`s. `proposing node num` is the number of `node`s that propose in the test, because we have unbalanced workload scenario (Fig.14 in our paper). `test mode` can be `0` for normal test and `1` for light workload test (Fig.13 in our paper).

### run client

After all `node`s have started and no more outputs prompt in that terminal, run in another terminal the following command to start `client`s
```
# start another terminal
# under BKR-SuperMA/test/script/
./clients.sh [running num] [proposing node num] [test mode]
```
The parameters must be the same with that in `node`.

### run coordinator

After all `client`s are ready, in the third terminal, run:

```
# start another terminal
# under BKR-SuperMA/coordinator/
./coor -b [batch size] -p [payload] -t [test time] -i [client rate]
```

The `coordinator/coor` will start `coordinator`.
`client rate` is the number of requests `client` send to `node` per 50 ms.
Now the experiment is started.

### stop

After `coordinator` runs out of time you specified, e.g. 30s, it will print the results on the terminal.

Then you can use `test/script/stop.sh` to stop all `node` and `client`:
```
# under BKR-SuperMA/test/script/
./stop.sh [running num]
```

## recommended parameters in our paper

We give the parameters we used to test BKR-SuperMA presented in our paper.

### fault-free 

#### n = 4 (Figure 4)

```
cd test/script
python3 aws.py nodes MyTumbler-artifacts 4
./create -n 4 -f 1 -p 4 -b 0
./deliverAll.sh 4

./nodes.sh 4 30000 1000 30 1 4 0

./clients.sh 4 4 0

cd coordinator
./coor -b 30000 -p 1000 -t 30 -i 50
```

To saturate the system and draw a curve, gradually increase `-i` the last command. The peak throughput appears around `-i 250`:

```
./coor -b 30000 -p 1000 -t 30 -i 250
```

#### n = 10 (Figure 5)

```
cd test/script
python3 aws.py nodes MyTumbler-artifacts 10
./create -n 10 -f 3 -p 10 -b 0
./deliverAll.sh 10

./nodes.sh 10 30000 1000 30 1 10 0

./clients.sh 10 10 0

cd coordinator
./coor -b 30000 -p 1000 -t 30 -i 50
```

To saturate the system and draw a curve, gradually increase `-i` the last command. The peak throughput appears around `-i 200`:

```
./coor -b 30000 -p 1000 -t 30 -i 200
```

#### n = 100 (Figure 6)

```
cd test/script
python3 aws.py nodes MyTumbler-artifacts 100
./create -n 100 -f 33 -p 100 -b 0
./deliverAll.sh 100

./nodes.sh 100 4000 1000 30 1 100 0

./clients.sh 100 100 0

cd coordinator
./coor -b 4000 -p 1000 -t 30 -i 10
```

To saturate the system and draw a curve, gradually increase `-i` the last command. The peak throughput appears around `-i 40`:

```
./coor -b 4000 -p 1000 -t 30 -i 40
```

### crash fault

#### n = 4 (Figure 7)

```
cd test/script
python3 aws.py nodes MyTumbler-artifacts 3
./create -n 4 -f 1 -p 3 -b 0
./deliverAll.sh 3

./nodes.sh 3 30000 1000 30 1 3 0

./clients.sh 3 3 0

cd coordinator
./coor -b 30000 -p 1000 -t 30 -i 50
```

To saturate the system and draw a curve, gradually increase `-i` the last command. The peak throughput appears around `-i 250`:

```
./coor -b 30000 -p 1000 -t 30 -i 250
```

#### n = 10 (Figure 8)

```
cd test/script
python3 aws.py nodes MyTumbler-artifacts 7
./create -n 10 -f 3 -p 7 -b 0
./deliverAll.sh 7

./nodes.sh 7 30000 1000 30 1 7 0

./clients.sh 7 7 0

cd coordinator
./coor -b 30000 -p 1000 -t 30 -i 50
```

To saturate the system and draw a curve, gradually increase `-i` the last command. The peak throughput appears around `-i 200`:

```
./coor -b 30000 -p 1000 -t 30 -i 200
```

#### n = 100 (Figure 9)

```
cd test/script
python3 aws.py nodes MyTumbler-artifacts 67
./create -n 100 -f 33 -p 67 -b 0
./deliverAll.sh 67

./nodes.sh 67 4000 1000 30 1 67 0

./clients.sh 67 67 0

cd coordinator
./coor -b 4000 -p 1000 -t 30 -i 10
```

To saturate the system and draw a curve, gradually increase `-i` the last command. The peak throughput appears around `-i 30`:

```
./coor -b 30000 -p 1000 -t 30 -i 30
```


### far-away node (Figure 11)
To test this scenario, please modify  `aws.py`, choose these regions (3 in asia and 1 in europe):
```
# for 4 nodes with 1 far away
regions = ["ap-east-1","ap-southeast-1","ap-northeast-1","eu-central-1"]
```
Then run:
```
cd test/script
python3 aws.py nodes MyTumbler-artifacts 4
./create -n 4 -f 1 -p 4 -b 0
./deliverAll.sh 4

./nodes.sh 4 30000 1000 60 1 4 0

./clients.sh 4 4 0

cd coordinator
./coor -b 30000 -p 1000 -t 60 -i 200
```

### light workload

#### n = 4 (Figure 13.a)

```
cd test/script
python3 aws.py nodes MyTumbler-artifacts 4
./create -n 4 -f 1 -p 4 -b 0
./deliverAll.sh 4

./nodes.sh 4 1 1000 60 1 4 1

./clients.sh 4 4 1

cd coordinator
./coor -b 1 -p 1000 -t 60 -i 1
```

#### n = 10 (Figure 13.b)

```
cd test/script
python3 aws.py nodes MyTumbler-artifacts 10
./create -n 10 -f 3 -p 10 -b 0
./deliverAll.sh 10

./nodes.sh 10 1 1000 60 1 10 1

./clients.sh 10 10 1

cd coordinator
./coor -b 1 -p 1000 -t 60 -i 1
```

### unbalanced workload

#### one node propose (Figure 14.a)

```
cd test/script
python3 aws.py nodes MyTumbler-artifacts 10
./create -n 10 -f 3 -p 10 -b 0
./deliverAll.sh 10

./nodes.sh 10 30000 1000 60 1 1 0

./clients.sh 10 1 0

cd coordinator
./coor -b 30000 -p 1000 -t 60 -i 250
```

#### three nodes propose (Figure 14.b)

You can skip the following step and directly run the test. Then are still 3 nodes proposing, but they are from only 2 regions. That would not make a big difference to the result.

But if you want to stick to the paper:
>`aws.py` gets instances information in the order of regions. With 10 nodes,  each region has 2 nodes, like this: [region_a1, region_a2, region_b1, region_b2, region_c1, ..., region_f2]. In our paper, however, we want three instances in different regions to propose, namely, region_a1, region_b1, region_c1. So after running `aws.py`, you have to manually modify `test/script/nodes.json` and `conf/nodes.txt`: swap the 2rd and 5th instance information.
>
>More specifically, the `test/script/nodes.json` originally is like this:
>```
>"nodes": [
>    {
>      "id": 1,
>      other information ...
>    },
>    {
>      "id": 2,
>      other information ...    ->    replace with id 5's information
>    },
>    {
>      "id": 3,
>      other information ...
>    },
>    {
>      "id": 4,
>      other information ...
>    },
>    {
>      "id": 5,
>      other information ...    ->    replace with id 2's information
>    },
>    ...
>]
>```
>You need to keep the "id" unchanged, but swap the other instance information of 2 and 5. 
>
>And the `conf/nodes.txt` should also be modified correspondingly. **Pay attention: only the ip is swapped; the port number should keep the original order**:
>```
>ip1:6001
>ip2:6002  ->  ip5:6002
>ip3:6003
>ip4:6004
>ip5:6005  ->  ip2:6005
>...
>```
>
>

Then:

```
cd test/script
./create -n 10 -f 3 -p 10 -b 0
./deliverAll.sh 10

./nodes.sh 10 30000 1000 60 1 3 0

./clients.sh 10 3 0

cd coordinator
./coor -b 30000 -p 1000 -t 60 -i 250
```


# License

(C) 2016-2023 Ant Group Co.,Ltd.  
SPDX-License-Identifier: Apache-2.0
