#!/bin/sh

# (C) 2016-2023 Ant Group Co.,Ltd.
# SPDX-License-Identifier: Apache-2.0
  
NAME=$1

ps -ef | grep "$NAME" | grep -v grep | awk '{print $2}' | xargs --no-run-if-empty kill

echo "done!" 
