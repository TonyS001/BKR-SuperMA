// (C) 2016-2023 Ant Group Co.,Ltd.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"BKR-SuperMA/common"
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strings"
	"sync"

	"github.com/gogo/protobuf/sortkeys"
)

type Coordinator struct {
	addr                  string
	mu                    sync.Mutex
	consensusLatency      map[uint32]uint64
	executionLatency      map[uint32]uint64
	executionLatency95    map[uint32]uint64
	consensusNumber       map[uint32]uint64
	executionNumber       map[uint32]uint64
	zeroNum               map[uint32]uint64
	total                 int
	curr                  int
	stop                  chan struct{}
	totalConsensusNum     uint64
	totalExecutionNum     uint64
	totalConsensusLatency uint64
	totalExecutionLatency uint64
	batchSize             int
	payload               int
	testTime              int
	clientRate            int
}

func main() {
	var batch, payload, test_time, interval int
	flag.IntVar(&batch, "b", 1000, "batch size")
	flag.IntVar(&payload, "p", 1000, "payload")
	flag.IntVar(&test_time, "t", 30, "test time")
	flag.IntVar(&interval, "i", 500, "client rate: req num per 50ms")
	flag.Parse()
	fmt.Printf("batch: %+v, payload: %+v, test time: %+v, client rate: %+v reqs per 50ms\n", batch, payload, test_time, interval)
	var dep common.Devp
	fileName2 := "../test/script/devip.json"
	depFile, err := os.Open(fileName2)
	if err != nil {
		panic(err)
	}
	defer depFile.Close()
	byteValue, _ := ioutil.ReadAll(depFile)
	json.Unmarshal(byteValue, &dep)

	coor := &Coordinator{
		addr:                  dep.PrivateIp + ":9000",
		consensusLatency:      make(map[uint32]uint64),
		executionLatency:      make(map[uint32]uint64),
		executionLatency95:    make(map[uint32]uint64),
		consensusNumber:       make(map[uint32]uint64),
		executionNumber:       make(map[uint32]uint64),
		zeroNum:               make(map[uint32]uint64),
		curr:                  0,
		totalConsensusNum:     0,
		totalExecutionNum:     0,
		totalConsensusLatency: 0,
		totalExecutionLatency: 0,
		stop:                  make(chan struct{}, 1),
		batchSize:             batch,
		payload:               payload,
		testTime:              test_time,
		clientRate:            interval,
	}
	startRpcServer(coor)

	fileName1 := "../conf/nodes.txt"
	f1, err1 := os.Open(fileName1)
	if err1 != nil {
		panic(err1)
	}
	buf1 := bufio.NewReader(f1)
	var nodes []string
	for {
		line, err := buf1.ReadString('\n')
		line = strings.TrimSpace(line)
		if err != nil {
			if err == io.EOF {
				nodes = append(nodes, line)
				break
			}
			panic(err)
		}
		nodes = append(nodes, line)
	}
	coor.total = len(nodes)
	for _, node := range nodes {
		go func(n string) {
			cli, err := rpc.DialHTTP("tcp", n)
			if err != nil {
				panic(err)
			}
			msg := &common.CoorStart{
				Batch:    coor.batchSize,
				Payload:  coor.payload,
				Interval: coor.clientRate,
			}
			var resp common.Response
			cli.Call("Client.OnStart", msg, &resp)
		}(node)
	}

	<-coor.stop

	fmt.Printf("execution average latency: %+v\n", coor.executionLatency)
	fmt.Printf("execution 95 latency: %+v\n", coor.executionLatency95)
	if coor.totalExecutionNum == 0 {
		fmt.Printf("total full-execution latency: 0\n")
	} else {
		fmt.Printf("total full-execution latency: %+v\n", coor.totalExecutionLatency/coor.totalExecutionNum)
	}
	fmt.Printf("total full-execution throughput: %+v\n", coor.totalExecutionNum/uint64(coor.testTime))
	fmt.Printf("-------------------------------------------\n")
}

func startRpcServer(server *Coordinator) {
	rpc.Register(server)
	rpc.HandleHTTP()
	listener, err := net.Listen("tcp", server.addr)
	if err != nil {
		panic(err)
	}
	go http.Serve(listener, nil)
}

func (c *Coordinator) Finish(req *common.CoorStatistics, resp *common.Response) error {
	c.mu.Lock()
	c.consensusLatency[req.ID] = req.ConsensusLatency
	c.consensusNumber[req.ID] = req.ConsensusNumber
	c.executionNumber[req.ID] = req.ExecutionNumber
	if req.ExecutionNumber != 0 {
		c.executionLatency[req.ID] = req.ExecutionLatency / req.ExecutionNumber
	} else {
		c.executionLatency[req.ID] = 0
	}
	c.zeroNum[req.ID] = req.Zero
	c.curr++
	c.totalConsensusNum += req.ConsensusNumber
	c.totalExecutionNum += req.ExecutionNumber
	c.totalConsensusLatency += req.ConsensusLatency
	c.totalExecutionLatency += req.ExecutionLatency
	sortkeys.Uint64s(req.LatencyMap)
	index := len(req.LatencyMap) * 95 / 100
	if req.ExecutionNumber != 0 {
		c.executionLatency95[req.ID] = req.LatencyMap[index]
	} else {
		c.executionLatency95[req.ID] = 0
	}
	if c.curr == c.total {
		c.stop <- struct{}{}
	}
	c.mu.Unlock()
	return nil
}
