# (C) 2016-2023 Ant Group Co.,Ltd.
# SPDX-License-Identifier: Apache-2.0

from re import findall, search
import sys
import re
import os
KeyStr = str(sys.argv[1])
for i in range(10):
    with open('../log/node'+ str(i+1)+'.log', 'r') as log:
        res = log.read().splitlines()
        f = open(str(i+1)+'-'+KeyStr+'.log', 'w+')
        num = 0
        for i, line in enumerate(res):
            if line.find(KeyStr) > 0:
                f.writelines(line+"\n")
                num = num + 1
        print(num)

f.close()
    
    
            