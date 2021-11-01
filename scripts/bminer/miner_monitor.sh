#!/bin/bash

clear
echo $1

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
. ${SCRIPT_DIR}/env.sh

pci_info=$(lspci -Dnn)

_nvidia_counts()
{
    nv_count=$(echo $pci_info | grep -E "VGA|3D controller" | grep "NVIDIA" | grep -v "nForce" | wc -l)
    echo $nv_count
}

_amd_counts()
{
    amd_count=$(echo $pci_info | grep -E "VGA|3D controller" | grep "Advanced Micro Devices" | grep -v "RS880" | wc -l)
    echo $amd_count
}

function getIpAddr(){
    ip=$(/usr/bin/ip route get 1 2>/dev/null | awk '{print $7;exit}')
    echo "$ip"
}

platform=nvidia
if [ $(_amd_counts) -gt $(_nvidia_counts) ];then
    platform=amd
fi

echo "Detect network ."
while true
do
    ip=$(getIpAddr)
    if [ -z "$ip" ];then
        printf "."
        sleep 2
        continue
    fi
    echo "$ip OK"
    break
done

echo "Detect gpus ."
while true
do
    ${BMINER_BIN_PATH}/gpu-control refresh
    gpus=$(${BMINER_BIN_PATH}/gpu-control detect 2>/dev/null)
    if [ -z "$gpus" ];then
        printf "."
        sleep 2
        continue
    fi
    echo "OK"
    echo "$gpus"
    break
done

if [ "$platform" == "nvidia" ] ;then
    echo "Detect Xorg service ."
    while true
    do
        x=$(ps aux | grep Xorg | grep -v grep | wc -l)
        if [ $x -eq 0 ];then
            printf "."
            sleep 2
            continue
        fi
        echo "OK"
        break
    done
fi

echo "Waiting for gpu overclock."

# when miner is not running
/usr/bin/supervisorctl status miner
if [ $? -ne 0 ]; then
    ${BMINER_BIN_PATH}/gpu-control oc force
    ${BMINER_BIN_PATH}/gpu-control fan
fi

echo "Waiting for miner start."

mine_info=$BMINER_DATA_PATH/mine_info.json
for x in 'addr' 'port' 'user' 'worker' 'proto'; do
    eval $x=$(jq -r ".${x}" $mine_info)
done

miner_name="PhoenixMiner"
miner_exe=${BMINER_MINERAPP_PATH}/${miner_name}/${miner_name}
echo "#!/bin/bash
cleanup() {
    ps -ef|grep ${miner_exe}|grep -v grep |awk '{print \$2}'|xargs kill
    exit
}
trap cleanup SIGINT SIGTERM
pminer=\$(ps -ef|grep ${miner_exe}|grep -v grep|wc -l)
if [ \$pminer -ne 0 ]; then
    echo 'miner already started!'
    exit
fi
${miner_exe} -wdog 0 -pool ${addr}:${port} -wal $user -worker $worker -proto 2 -logfile ${BMINER_RUN_PATH}/miner.log
# t-rex
# ${miner_exe} -a ethash -d 0,1,2,3,4,5,6,7 -o ${proto}://${addr}:${port} -u $user -w $worker -p x -l ${BMINER_RUN_PATH}/miner.log
# nbminer
# ${miner_exe} -a ethash -o ${proto}://${addr}:${port} -u ${user}.${worker} --log-file ${BMINER_RUN_PATH}/miner.log
" > $BMINER_BIN_PATH/start_miner.sh

/usr/bin/supervisorctl start miner

while true
do
    x=$(ps aux | grep start_miner.sh | grep -v grep | wc -l)
    if [ $x -eq 0 ];then
        printf "."
        sleep 2
        continue
    fi
    echo "OK"
    break
done

while true; do
    tail -f ${BMINER_RUN_PATH}/miner.log
    sleep 15
done
