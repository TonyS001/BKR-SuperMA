// (C) 2016-2023 Ant Group Co.,Ltd.
// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"BKR-SuperMA/common"
	"BKR-SuperMA/crypto"
	"BKR-SuperMA/logger"
	"BKR-SuperMA/network"
	"bytes"
	"errors"
	"net"
	"net/http"
	"net/rpc"
	"sort"
	"time"

	"github.com/gogo/protobuf/proto"
)

type Node struct {
	cfg                *common.Config
	network            network.NetWork
	networkPayList     []network.NetWork
	peers              map[uint32]common.Peer
	stop               bool
	nextProposeSeq     uint32
	nextExecSeq        uint32
	instances          map[uint32]map[uint32]*SuperMA
	baMsgChans         map[uint32]map[uint32]chan *common.Message
	hashToPayloads     map[uint32]map[string][]byte
	hashToVals         map[uint32]map[string][]*common.Message
	dispatchChan       chan *common.Message
	msgChan            chan *common.Message
	supermaFinishChan  chan *common.SuperMAResult
	proposeChan        chan *common.BlockInfo
	reProposeChan      chan *common.BlockInfo
	tryProposeChan     chan uint32
	consensusLatencies []uint64
	executionLatencies []uint64
	zeroNum            int
	startChan          chan struct{}
	startProposeChan   chan struct{}
	logger             logger.Logger
	ipList             []string
	mergeMsg           map[uint32]map[string]common.PayloadIds
	sliceNum           uint32
	currBatch          *common.Batch
	clientChan         chan *common.ClientReq
	connection         *rpc.Client
	ifPropose          int
	testMode           int
	reqNum             int
	proposeflag        bool
	interval           int
	startId            int32
	blockInfos         []*common.BlockInfo
	baSuccess          map[uint32]map[uint32]string
	baZero             map[uint32]map[uint32]struct{}
	haveEndorsed       map[uint32]map[uint32]struct{}
	haveVoteZero       map[uint32]struct{}
}

func NewNode(cfg *common.Config, peers map[uint32]common.Peer, logger logger.Logger, conn uint32, ifBussy int, testMode int) *Node {
	crypto.Init()

	node := &Node{
		cfg:                cfg,
		network:            nil,
		networkPayList:     nil,
		peers:              peers,
		stop:               false,
		nextProposeSeq:     0,
		nextExecSeq:        0,
		instances:          make(map[uint32]map[uint32]*SuperMA),
		baMsgChans:         make(map[uint32]map[uint32]chan *common.Message),
		hashToPayloads:     make(map[uint32]map[string][]byte),
		consensusLatencies: make([]uint64, 0),
		executionLatencies: make([]uint64, 0),
		hashToVals:         make(map[uint32]map[string][]*common.Message),
		dispatchChan:       make(chan *common.Message, 200000),
		msgChan:            make(chan *common.Message, 200000),
		supermaFinishChan:  make(chan *common.SuperMAResult, 200),
		proposeChan:        make(chan *common.BlockInfo, 10),
		reProposeChan:      make(chan *common.BlockInfo, 10),
		tryProposeChan:     make(chan uint32, 10),
		startChan:          make(chan struct{}, 1),
		startProposeChan:   make(chan struct{}, 1),
		clientChan:         make(chan *common.ClientReq, 1000),
		logger:             logger,
		ipList:             nil,
		currBatch:          new(common.Batch),
		reqNum:             0,
		ifPropose:          ifBussy,
		testMode:           testMode,
		proposeflag:        true,
		mergeMsg:           make(map[uint32]map[string]common.PayloadIds),
		sliceNum:           conn,
		zeroNum:            0,
		blockInfos:         make([]*common.BlockInfo, 0),
		baSuccess:          make(map[uint32]map[uint32]string),
		baZero:             make(map[uint32]map[uint32]struct{}),
		haveEndorsed:       make(map[uint32]map[uint32]struct{}),
		haveVoteZero:       make(map[uint32]struct{}),
	}

	node.startId = 1

	for _, peer := range peers {
		node.ipList = append(node.ipList, peer.Addr[0:(len(peer.Addr)-5)])
	}

	node.network = network.NewNoiseNetWork(node.cfg.ID, node.cfg.Addr, node.peers, node.msgChan, node.dispatchChan, node.logger, false, 0)
	for i := uint32(0); i < conn; i++ {
		node.networkPayList = append(node.networkPayList, network.NewNoiseNetWork(node.cfg.ID, node.cfg.Addr, node.peers, node.msgChan, node.dispatchChan, node.logger, true, i+1))
	}
	return node
}

func (n *Node) Run() {
	n.startRpcServer()
	n.network.Start()
	for _, networkPay := range n.networkPayList {
		networkPay.Start()
	}
	go n.dispatch()
	go n.proposeLoop()
	n.mainLoop()
}

func (n *Node) startRpcServer() {
	rpc.Register(n)
	rpc.HandleHTTP()
	listener, err := net.Listen("tcp", n.cfg.RpcServer)
	if err != nil {
		panic(err)
	}
	go http.Serve(listener, nil)
}

func (n *Node) OnStart(msg *common.CoorStart, resp *common.Response) error {
	n.startChan <- struct{}{}
	return nil
}

func (n *Node) Request(req *common.ClientReq, resp *common.ClientResp) error {
	if req.StartId == 1 {
		n.interval = int(req.ReqNum)
		n.startChan <- struct{}{}
		n.startProposeChan <- struct{}{}
	}
	n.clientChan <- req
	return nil
}

func (n *Node) mainLoop() {
	<-n.startChan

	conn, err := rpc.DialHTTP("tcp", n.cfg.ClientServer)
	if err != nil {
		panic(err)
	}
	n.connection = conn

	timer := time.NewTimer(time.Second * time.Duration(n.cfg.Time))
	for {
		if n.stop {
			return
		}
		select {
		case msg := <-n.msgChan:
			n.handleMessage(msg)
		case res := <-n.supermaFinishChan:
			n.onSuperMAFinish(res)
		case block := <-n.proposeChan:
			n.propose(false, block)
		case block := <-n.reProposeChan:
			n.propose(true, block)
		case <-timer.C:
			n.StopClient()
			n.stop = true
		}
	}
}

func (n *Node) proposeLoop() {
	if n.testMode == 1 {
		n.cfg.MaxBatchSize = 1
	}

	<-n.startProposeChan

	timer := time.NewTimer(time.Millisecond * time.Duration(n.cfg.MaxWaitTime))
	for {
		select {
		case <-timer.C:
			if n.proposeflag {
				n.getBatch()
				n.proposeflag = false
			}
		case req := <-n.clientChan:
			n.currBatch.Reqs = append(n.currBatch.Reqs, req)
			n.reqNum += n.interval
		case <-n.tryProposeChan:
			n.getBatch()
			n.proposeflag = false
		}
	}
}

func (n *Node) getBatch() {
	if n.reqNum <= n.cfg.MaxBatchSize {
		payloadBytes, _ := proto.Marshal(n.currBatch)
		n.currBatch.Reset()
		block := &common.BlockInfo{
			StartID:   n.startId,
			ReqNum:    int32(n.reqNum),
			StartTime: time.Now().UnixNano() / 1000000,
			Hash:      crypto.Hash(payloadBytes),
			Payload:   payloadBytes,
		}
		n.startId += int32(n.reqNum)
		n.reqNum = 0
		n.blockInfos = append(n.blockInfos, block)
		n.proposeChan <- block
	} else {
		reqs := n.currBatch.Reqs[0 : n.cfg.MaxBatchSize/n.interval]
		n.currBatch.Reqs = n.currBatch.Reqs[n.cfg.MaxBatchSize/n.interval:]
		clientreqs := new(common.Batch)
		clientreqs.Reqs = reqs
		payloadBytes, _ := proto.Marshal(clientreqs)
		block := &common.BlockInfo{
			StartID:   n.startId,
			ReqNum:    int32(n.cfg.MaxBatchSize),
			StartTime: time.Now().UnixNano() / 1000000,
			Hash:      crypto.Hash(payloadBytes),
			Payload:   payloadBytes,
		}
		n.blockInfos = append(n.blockInfos, block)
		n.startId += int32(n.cfg.MaxBatchSize)
		n.reqNum -= n.cfg.MaxBatchSize
		n.proposeChan <- block
	}
}

func (n *Node) dispatch() {
	for {
		select {
		case msg := <-n.dispatchChan:
			if !n.existInstance(msg.Seq, msg.Sender) {
				n.startInstance(msg)
			} else {
				n.baMsgChans[msg.Sender][msg.Seq] <- msg
			}
		}
	}
}

func (n *Node) StopClient() {
	st := &common.NodeBack{
		Zero:   uint32(n.zeroNum),
		NodeID: n.cfg.ID,
		Addr:   n.cfg.Coordinator,
	}
	var resp common.Response
	n.connection.Call("Client.NodeFinish", st, &resp)
}

func (n *Node) propose(redo bool, block *common.BlockInfo) {
	if !redo {
		n.broadcastPayload(block)
	}

	proposal := &common.Message{
		Round:  0,
		Sender: n.cfg.ID,
		Type:   common.Message_BVAL,
		Seq:    n.nextProposeSeq,
		Hash:   block.Hash,
	}
	msgBytes, err := proto.Marshal(proposal)
	if err != nil {
		n.logger.Error("marshal val failed", err)
		return
	}
	proposal.Signature = crypto.Sign(n.cfg.PrivKey, msgBytes)

	proposal.From = n.cfg.ID
	proposal.Type = common.Message_VAL
	if n.hashToVals[n.cfg.ID] == nil {
		n.hashToVals[n.cfg.ID] = make(map[string][]*common.Message)
	}
	n.hashToVals[n.cfg.ID][proposal.Hash] = append(n.hashToVals[n.cfg.ID][proposal.Hash], proposal)
	n.network.BroadcastMessage(proposal)
	n.dispatchChan <- proposal
	n.logger.Infof("propose %v", n.nextProposeSeq)
	n.nextProposeSeq++
}

func (n *Node) broadcastPayload(block *common.BlockInfo) {
	sliceLength := len(block.Payload) / int(n.sliceNum)
	for i := uint32(0); i < n.sliceNum; i++ {
		msgSlice := &common.Message{
			From:            n.cfg.ID,
			Round:           0,
			Sender:          n.cfg.ID,
			Type:            common.Message_PAYLOAD,
			Hash:            block.Hash,
			TotalPayloadNum: n.sliceNum,
			PayloadSlice:    i + 1,
		}
		if i < (n.sliceNum - 1) {
			msgSlice.Payload = block.Payload[i*uint32(sliceLength) : (i+1)*uint32(sliceLength)]
		} else {
			msgSlice.Payload = block.Payload[i*uint32(sliceLength):]
		}
		n.networkPayList[i].BroadcastMessage(msgSlice)
	}
	if n.hashToPayloads[n.cfg.ID] == nil {
		n.hashToPayloads[n.cfg.ID] = make(map[string][]byte)
	}
	n.hashToPayloads[n.cfg.ID][block.Hash] = block.Payload
}

func (n *Node) handleMessage(msg *common.Message) {
	switch msg.Type {
	case common.Message_PAYLOAD:
		n.onReceivePayload(msg)
	case common.Message_VAL:
		n.onReceiveVal(msg)
	default:
		n.logger.Error("invalid msg type", errors.New("error in msg dispatch"))
	}
}

func (n *Node) onReceiveVal(msg *common.Message) {
	if msg.Seq < n.nextExecSeq {
		return
	}

	n.dispatchChan <- msg
	if n.hashToVals[msg.Sender] == nil {
		n.hashToVals[msg.Sender] = make(map[string][]*common.Message)
	}
	n.hashToVals[msg.Sender][msg.Hash] = append(n.hashToVals[msg.Sender][msg.Hash], msg)

	if _, ok1 := n.haveEndorsed[msg.Sender]; ok1 {
		if _, ok2 := n.haveEndorsed[msg.Sender][msg.Seq]; ok2 {
			return
		}
	}

	if _, ok := n.hashToPayloads[msg.Sender][msg.Hash]; ok {
		endorse := &common.Message{
			Seq:    msg.Seq,
			Sender: msg.Sender,
			Type:   common.Message_ENDORSE,
			Hash:   msg.Hash,
		}
		n.dispatchChan <- endorse

		if n.haveEndorsed[msg.Sender] == nil {
			n.haveEndorsed[msg.Sender] = make(map[uint32]struct{})
		}
		n.haveEndorsed[msg.Sender][msg.Seq] = struct{}{}
	}
}

func (n *Node) onReceivePayload(msgSlice *common.Message) {
	if n.mergeMsg[msgSlice.Sender] == nil {
		n.mergeMsg[msgSlice.Sender] = make(map[string]common.PayloadIds)
	}
	n.mergeMsg[msgSlice.Sender][msgSlice.Hash] = append(n.mergeMsg[msgSlice.Sender][msgSlice.Hash], common.PayloadId{Id: msgSlice.PayloadSlice, Payload: msgSlice.Payload})
	if len(n.mergeMsg[msgSlice.Sender][msgSlice.Hash]) == int(n.sliceNum) {
		sort.Sort(n.mergeMsg[msgSlice.Sender][msgSlice.Hash])
		var buffer bytes.Buffer
		for _, ps := range n.mergeMsg[msgSlice.Sender][msgSlice.Hash] {
			buffer.Write(ps.Payload)
		}
		if _, ok := n.hashToPayloads[msgSlice.Sender][msgSlice.Hash]; ok {
			return
		}
		if n.hashToPayloads[msgSlice.Sender] == nil {
			n.hashToPayloads[msgSlice.Sender] = make(map[string][]byte)
		}
		n.hashToPayloads[msgSlice.Sender][msgSlice.Hash] = buffer.Bytes()

		if n.checkCanExecute() {
			n.stepNextSequence()
			return
		}

		if _, ok := n.hashToVals[msgSlice.Sender][msgSlice.Hash]; ok {
			trueIndex := -1
			for index, val := range n.hashToVals[msgSlice.Sender][msgSlice.Hash] {
				if val.Seq < n.nextExecSeq {
					continue
				}
				if _, ok := n.baSuccess[val.Seq][val.Sender]; ok {
					continue
				}
				if _, ok := n.baZero[val.Seq][val.Sender]; ok {
					continue
				}
				if _, ok1 := n.haveEndorsed[val.Sender]; ok1 {
					if _, ok2 := n.haveEndorsed[val.Sender][val.Seq]; ok2 {
						continue
					}
				}
				trueIndex = index
			}
			if trueIndex != -1 {
				val := n.hashToVals[msgSlice.Sender][msgSlice.Hash][trueIndex]
				endorse := &common.Message{
					Seq:    val.Seq,
					Sender: val.Sender,
					Type:   common.Message_ENDORSE,
					Hash:   val.Hash,
				}
				n.dispatchChan <- endorse
				if n.haveEndorsed[val.Sender] == nil {
					n.haveEndorsed[val.Sender] = make(map[uint32]struct{})
				}
				n.haveEndorsed[val.Sender][val.Seq] = struct{}{}
			}
		}
	}
}

func (n *Node) onSuperMAFinish(res *common.SuperMAResult) {
	if res.Hash == "0" && res.Key.Sender == n.cfg.ID {
		n.zeroNum++
	}

	if res.Hash != "0" {
		if res.Key.Sender == n.cfg.ID {
			latency := uint64(time.Now().UnixNano()/1000000) - uint64(n.blockInfos[len(n.consensusLatencies)].StartTime)
			n.consensusLatencies = append(n.consensusLatencies, latency)
			n.logger.Debugf("my superma %v-%v finished %v ms", res.Key.Seq, res.Key.Sender, latency)
		}

		if _, ok := n.baSuccess[res.Key.Seq]; !ok {
			n.baSuccess[res.Key.Seq] = make(map[uint32]string)
		}
		n.baSuccess[res.Key.Seq][res.Key.Sender] = res.Hash
		if _, ok := n.haveVoteZero[res.Key.Seq]; !ok {
			if uint32(len(n.baSuccess[res.Key.Seq])) >= n.cfg.N-n.cfg.F {
				n.haveVoteZero[res.Key.Seq] = struct{}{}
				n.voteZero(res.Key.Seq)
			}
		}
	} else {
		if _, ok := n.baZero[res.Key.Seq]; !ok {
			n.baZero[res.Key.Seq] = make(map[uint32]struct{})
		}
		n.baZero[res.Key.Seq][res.Key.Sender] = struct{}{}
	}

	if n.checkCanExecute() {
		n.stepNextSequence()
	}
}

func (n *Node) checkCanExecute() bool {
	if uint32(len(n.baSuccess[n.nextExecSeq])+len(n.baZero[n.nextExecSeq])) < n.cfg.N {
		return false
	}

	for id, hash := range n.baSuccess[n.nextExecSeq] {
		if _, ok := n.hashToPayloads[id][hash]; !ok {
			return false
		}
	}
	return true
}

func (n *Node) stepNextSequence() {
	if _, ok := n.baSuccess[n.nextExecSeq][n.cfg.ID]; ok {
		index := len(n.executionLatencies)
		latency := uint64(time.Now().UnixNano()/1000000) - uint64(n.blockInfos[index].StartTime)
		n.executionLatencies = append(n.executionLatencies, latency)
		n.logger.Debugf("execute %v with %v reqs used %v ms", n.nextExecSeq, n.blockInfos[index].ReqNum, latency)
		st := &common.NodeBack{
			StartID:     uint32(n.blockInfos[index].StartID),
			ReqNum:      uint32(n.blockInfos[index].ReqNum),
			SupermaTime: n.consensusLatencies[index],
			NodeID:      0,
		}
		var resp common.Response
		n.connection.Call("Client.NodeFinish", st, &resp)
		n.tryProposeChan <- uint32(len(n.consensusLatencies))
	}
	if _, ok := n.baZero[n.nextExecSeq][n.cfg.ID]; ok {
		if len(n.blockInfos) <= len(n.consensusLatencies) {
			n.tryProposeChan <- n.nextExecSeq + 1
		} else {
			n.reProposeChan <- n.blockInfos[len(n.consensusLatencies)]
		}
	}
	n.nextExecSeq++
}

func (n *Node) voteZero(seq uint32) {
	for id := uint32(1); id <= n.cfg.N; id++ {
		if _, ok := n.baSuccess[seq][id]; !ok {
			zero := &common.Message{
				Seq:    seq,
				Sender: id,
				Type:   common.Message_VOTEZERO,
			}
			n.dispatchChan <- zero
		}
	}
}

func (n *Node) existInstance(seq uint32, sender uint32) bool {
	if _, ok := n.instances[sender]; !ok {
		return false
	}
	_, ok := n.instances[sender][seq]
	return ok
}

func (n *Node) startInstance(msg *common.Message) {
	if n.instances[msg.Sender] == nil {
		n.baMsgChans[msg.Sender] = make(map[uint32]chan *common.Message)
		n.instances[msg.Sender] = make(map[uint32]*SuperMA)
	}
	n.baMsgChans[msg.Sender][msg.Seq] = make(chan *common.Message, 1000)
	n.baMsgChans[msg.Sender][msg.Seq] <- msg
	n.instances[msg.Sender][msg.Seq] = NewSuperMA(msg.Sender, msg.Seq, n.peers,
		n.network, n.cfg, n.supermaFinishChan, n.baMsgChans[msg.Sender][msg.Seq], n.logger)
	go n.instances[msg.Sender][msg.Seq].Run()
}
