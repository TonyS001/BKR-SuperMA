#!/bin/bash

# (C) 2016-2023 Ant Group Co.,Ltd.
# SPDX-License-Identifier: Apache-2.0

NODE=$1

BATCHSIZE=$2

PAYLOAD=$3

TIME=$4

SLICE=$5

PROPOSE=$6

MODE=$7
    
cd ..

rm *.log

./mod --size=$BATCHSIZE --payload=$PAYLOAD --node=$NODE --time=$TIME

./BKRSuperMA -c ./conf/$NODE.json -n $SLICE -p $PROPOSE -m $MODE >$NODE.log 2>&1

sleep 100

