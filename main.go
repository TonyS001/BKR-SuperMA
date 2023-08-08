// (C) 2016-2023 Ant Group Co.,Ltd.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"BKR-SuperMA/common"
	"BKR-SuperMA/consensus"
	"BKR-SuperMA/logger"
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"
	"strconv"
)

func main() {
	configFile := flag.String("c", "", "config file")
	payloadCon := flag.Int("n", 2, "payload connection num")
	ifBussy := flag.Int("p", 1, "if propose")
	testMode := flag.Int("m", 0, "test mode: 0 for normal, 1 for light")
	flag.Parse()
	jsonFile, err := os.Open(*configFile)
	if err != nil {
		panic(err)
	}
	defer jsonFile.Close()
	byteValue, _ := ioutil.ReadAll(jsonFile)
	c := new(common.ConfigFile)
	json.Unmarshal(byteValue, c)
	peers := make(map[uint32]common.Peer)
	for _, p := range c.Peers {
		peers[p.ID] = p
	}
	node := consensus.NewNode(&c.Cfg, peers, logger.NewZeroLogger("./node"+strconv.FormatUint(uint64(c.Cfg.ID), 10)+".log"), uint32(*payloadCon), *ifBussy, *testMode)
	node.Run()
}
