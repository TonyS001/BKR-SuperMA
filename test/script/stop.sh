#!/bin/bash

# (C) 2016-2023 Ant Group Co.,Ltd.
# SPDX-License-Identifier: Apache-2.0

NUM=$1

filename="nodes"
PNAME1="BKRSuperMA"
PNAME2="client"

for(( i = 0 ; i < NUM ; i++)); do

        {
   
        host1=$(jq  .$filename'['$i'].host'  $filename.json)

        host=${host1//\"/}

        port1=$(jq  .$filename'['$i'].port'  $filename.json)

        port=${port1//\"/}
        
        user1=$(jq  .$filename'['$i'].user'  $filename.json)

        user=${user1//\"/}

        key1=$(jq  .$filename'['$i'].keypath'  $filename.json)

        key=${key1//\"/}
        
        id1=$(jq  '.nodes['$i'].id'  nodes.json)

        id=${id1//\"/}

        node="node"$id
        
        expect -c "

        set timeout -1

        spawn scp -i $key $user@$host:BKR-SuperMA/$node.log ../../log/

        expect 100%

        exit

       "
        
	expect <<-END

        set timeout -1

        spawn ssh -oStrictHostKeyChecking=no -i $key $user@$host -p $port "cd BKR-SuperMA/script;chmod +x close_p.sh;sudo ./close_p.sh $PNAME1"
          
        expect EOF
          
	END

        expect <<-END

        set timeout -1

        spawn ssh -oStrictHostKeyChecking=no -i $key $user@$host -p $port "cd BKR-SuperMA/script;chmod +x close_p.sh;sudo ./close_p.sh $PNAME2"
          
        expect EOF
          
	END

        } &
       
done

wait

