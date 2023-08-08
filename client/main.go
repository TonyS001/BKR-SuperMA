// (C) 2016-2023 Ant Group Co.,Ltd.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"BKR-SuperMA/common"
	"math/rand"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strconv"
	"time"
)

type Client struct {
	publicAddr         string
	privateAddr        string
	interval           int
	reqId              int32
	nodeId             uint32
	startChan          chan struct{}
	sendChan           chan struct{}
	stopChan           chan string
	zeroNum            uint32
	payload            int
	startTime          []uint64
	consensusLatencies []uint64
	executionLatencies []uint64
	clientLatencies    []uint64
	finishNum          uint64
	allTime            uint64
	id                 int64
	ifWork             string
	testMode           string
}

func main() {
	id, _ := strconv.ParseInt(os.Args[3], 10, 64)
	rand.Seed(time.Now().UnixNano())
	client := &Client{
		publicAddr:         os.Args[1],
		privateAddr:        os.Args[2],
		id:                 id,
		reqId:              1,
		startChan:          make(chan struct{}, 1),
		sendChan:           make(chan struct{}, 1),
		stopChan:           make(chan string, 1),
		startTime:          make([]uint64, 0),
		consensusLatencies: make([]uint64, 0),
		executionLatencies: make([]uint64, 0),
		clientLatencies:    make([]uint64, 0),
		zeroNum:            0,
		finishNum:          0,
		allTime:            0,
		ifWork:             os.Args[4],
		testMode:           os.Args[5],
	}
	startRpcServer(client)

	cli, err := rpc.DialHTTP("tcp", client.publicAddr)
	if err != nil {
		panic(err)
	}

	<-client.startChan

	payload := make([]byte, client.payload*client.interval)
	if client.ifWork == "off" {
		for {
			req := &common.ClientReq{
				Id:      client.id,
				StartId: client.reqId,
				ReqNum:  0,
				Payload: payload,
			}
			client.reqId += int32(client.interval)
			var resp common.ClientResp
			go cli.Call("Node.Request", req, &resp)
			time.Sleep(time.Millisecond * 1800000)
		}
	}
	for {
		req := &common.ClientReq{
			Id:      client.id,
			StartId: client.reqId,
			ReqNum:  int32(client.interval),
			Payload: payload,
		}
		client.reqId += int32(client.interval)
		var resp common.ClientResp
		cli.Call("Node.Request", req, &resp)
		client.startTime = append(client.startTime, uint64(time.Now().UnixNano()/1000000))
		if client.testMode == "light" {
			wait := rand.Intn(2 * 1000)
			time.Sleep(time.Millisecond * time.Duration(wait))
		} else {
			time.Sleep(time.Millisecond * 50)
		}
	}
}

func startRpcServer(server *Client) {
	rpc.Register(server)
	rpc.HandleHTTP()
	listener, err := net.Listen("tcp", server.privateAddr)
	if err != nil {
		panic(err)
	}
	go http.Serve(listener, nil)
}

func (cl *Client) OnStart(msg *common.CoorStart, resp *common.Response) error {
	cl.payload = msg.Payload
	cl.interval = msg.Interval
	if cl.testMode == "light" {
		cl.interval = 1
	}
	cl.startChan <- struct{}{}
	return nil
}

func (cl *Client) NodeFinish(msg *common.NodeBack, resp *common.Response) error {
	if msg.NodeID == 0 {
		cl.finishNum += uint64(msg.ReqNum)
		nowTime := uint64(time.Now().UnixNano() / 1000000)
		thisLatency := uint64(0)
		if cl.testMode == "normal" {
			for i := 0; i < int(msg.ReqNum)/cl.interval; i++ {
				cl.allTime += ((nowTime - cl.startTime[msg.StartID/uint32(cl.interval)+uint32(i)]) * uint64(cl.interval))
				thisLatency += ((nowTime - cl.startTime[msg.StartID/uint32(cl.interval)+uint32(i)]) * uint64(cl.interval))
			}
			cl.consensusLatencies = append(cl.consensusLatencies, msg.SupermaTime)
			if msg.ReqNum == 0 {
				cl.clientLatencies = append(cl.clientLatencies, 0)
			} else {
				cl.clientLatencies = append(cl.clientLatencies, thisLatency/uint64(msg.ReqNum))
			}
		} else {
			cl.consensusLatencies = append(cl.consensusLatencies, msg.SupermaTime)
			if msg.ReqNum != 0 {
				for i := 0; i < int(msg.ReqNum)/cl.interval; i++ {
					cl.allTime += (nowTime - cl.startTime[msg.StartID-1+uint32(i)])
					thisLatency += (nowTime - cl.startTime[msg.StartID-1+uint32(i)])
				}
				cl.clientLatencies = append(cl.clientLatencies, thisLatency/uint64(msg.ReqNum))
			}
		}
	} else {
		cl.nodeId = msg.NodeID
		cl.zeroNum = msg.Zero
		cl.Stop(msg.Addr)
	}
	return nil
}

func (cl *Client) Stop(addr string) {
	conn, err := rpc.DialHTTP("tcp", addr)
	if err != nil {
		panic(err)
	}
	totalConsensusLatency := uint64(0)
	for _, l := range cl.consensusLatencies {
		totalConsensusLatency += l
	}
	st := &common.CoorStatistics{
		Zero:            uint64(cl.zeroNum),
		ConsensusNumber: uint64(len(cl.consensusLatencies)),
		ExecutionNumber: cl.finishNum,
		ID:              uint32(cl.nodeId),
		LatencyMap:      cl.clientLatencies,
	}
	if len(cl.consensusLatencies) == 0 {
		st.ConsensusLatency = 0
	} else {
		st.ConsensusLatency = totalConsensusLatency / uint64(len(cl.consensusLatencies))
	}
	if cl.finishNum == 0 {
		st.ExecutionLatency = 0
	} else {
		st.ExecutionLatency = cl.allTime
	}
	var resp common.Response
	conn.Call("Coordinator.Finish", st, &resp)
}
