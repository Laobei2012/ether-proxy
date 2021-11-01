#!/bin/bash

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
. ${SCRIPT_DIR}/env.sh

. ${SCRIPT_DIR}/_inc_hw.sh

function getWorkId(){
    wid=
    mine_info=$BMINER_DATA_PATH/mine_info.json

    if [ -f "$mine_info" ]; then
        for x in 'addr' 'port' 'user' 'worker' 'proto'; do
            eval $x=$(jq -r ".${x}" $mine_info)
        done
        wid=${worker}@${proto}://${addr}:${port}
        # wid=$(jq -r ".worker" $mine_info)
    fi

    echo $wid
}


function get_platform(){
    platform=$(cat ${BMINER_RUN_PATH}/platform)
    echo $platform
}

function get_version() {
    ver=$(cat ${BMINER_BIN_PATH}/release)
    
    echo "${ver}"
}

function print_help(){
    
    work_id=$(getWorkId)
    host_addr=$(getMACAddr)/$(getHostAddr)
    platform=$(get_platform)
    ver=$(get_version)
    cur_time=$(date +'%F %T')
    ${BMINER_BIN_PATH}/gpu-control refresh
    gpustatus=$(${BMINER_BIN_PATH}/gpu-control status)
    ENG=`cat <<EOF
\033[1;33m[${cur_time}]\033[0m
======================================================================
Worker ID   : ${work_id}
IP/MAC      : ${host_addr}
Platform    : ${platform}
Version     : ${ver}

GPUS:
${gpustatus}

======================================================================
EOF
    `
    echo -ne "$ENG\n"
}

export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/opt/bminer

# print_help

while true; do
    clear
    print_help
    sleep 15
done