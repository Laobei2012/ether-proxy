#!/bin/bash

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
. ${SCRIPT_DIR}/env.sh

. ${SCRIPT_DIR}/_inc_hw.sh

mac=$(getMACAddr)
localip=$(getHostAddr)
publicip=$(getPublicAddr)
cpu=$(getCPUInfo)
mem=$(getMEMInfo)
disk=$(getDiskInfo)
uptime=$(getUptime)
os=$(getOSVersion)

echo '{}'|jq -c --arg mac "$mac" --arg localip "$localip" --arg publicip "$publicip" --arg cpu "$cpu" --arg mem "$mem" --arg disk "$disk" --arg uptime "$uptime" --arg os "$os" '.mac=$mac|.localip=$localip|.publicip=$publicip|.cpu=$cpu|.mem=$mem|.disk=$disk|.uptime=$uptime|.os=$os'

