#!/bin/bash

platform=$1
addr=$2
port=$3
user=$4
worker=$5
proto=$6

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
. ${SCRIPT_DIR}/env.sh


#${BMINER_MINERAPP_PATH}/PhoenixMiner/PhoenixMiner -$platform -log 1 -di 01234567 -pool ${addr}:${port} -wal $worker -proto ${proto} -logfile ${BMINER_RUN_PATH}/miner.log

#./t-rex -a ethash -o stratum+tcp://cn.sparkpool.com:3333 -u 0x10d2b7ce00a6f8d7bab27003bde720e3d1553eb3 -p x -w w3070

${BMINER_MINERAPP_PATH}/t-rex/t-rex -a ethash -d 0,1,2,3,4,5,6,7 -o ${proto}://${addr}:${port} -u $user -w $worker -p x -l ${BMINER_RUN_PATH}/miner.log --pl 129
