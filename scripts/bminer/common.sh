_sourced_="__common_sourced_$$__"

if [ -z "${!_sourced_}" ]; then
    eval "$_sourced_=1"
else
    return 0
fi


pci_info=${PCI_INFO-"${BMINER_RUN_PATH}/pci_info"}
gpu_info_amd_file=${AMDMEMINFO-"${BMINER_RUN_PATH}/amdmeminfo"}
gpu_info_nv_file=${AMDMEMINFO-"${BMINER_RUN_PATH}/nvinfo"}
gpu_info_static=${GPU_INFO_STATIC-"${BMINER_RUN_PATH}/gpu_info_static"}
gpu_info_status=${GPU_INFO_STATUS-"${BMINER_RUN_PATH}/gpu_info_status"}
device_info=${DEVICE_INFO-"${BMINER_RUN_PATH}/device_info"}
driver_try_counts=${DEVICE_INFO-"${BMINER_RUN_PATH}/driver_try_counts"}
gpu_try_counts=5
#config from agent
# dev_ctrl=${GPU_CONTROL-"/etc/dev_ctrl"}

platform=""

_log(){
    echo "`date \"+%Y-%m-%d %H:%M:%S\"` $*" >> ${BMINER_RUN_PATH}/bminer.log
}

_lock(){
    ## Test the lock
    LOCKFILE=$1
    if [ -e ${LOCKFILE} ] && kill -0 `cat ${LOCKFILE}` 2> /dev/null ; then
        # _log "Script already running. bye!"
        return 1 
    fi

    ## Set the lock 
    echo $$ > ${LOCKFILE}
    return 0
}

# $1 为目标次数， 达到后返回1， 未达到返回0
_driver_add_try(){
    target_counts=$1
    if [ -f $driver_try_counts ];then
        c=$(cat $driver_try_counts)
        [[ $c -gt $target_counts ]] && return 1
        echo $((c+1)) > $driver_try_counts
        _log "_driver_try counts: $((c+1))"
        return 0
    fi
    echo 0 > $driver_try_counts
    return 0
}

_driver_notry(){
    target_counts=$1
    c=0
    test -f $driver_try_counts && c=$(cat $driver_try_counts)
    echo $((c+target_counts)) > $driver_try_counts
    return 0
}

# $1 为目标次数， 达到后返回1， 未达到返回0
_driver_try(){
   target_counts=$1
   c=$(cat $driver_try_counts)
   if [[ $target_counts -le $c ]];then
        return 1
   fi
   return 0
}

_pci_info()
{
    [[ ! -e $pci_info ]] && lspci -Dnn > $pci_info
    cat $pci_info
}

_nvidia_counts()
{
    nv_count=$(_pci_info | grep -E "VGA|3D controller" | grep "NVIDIA" | grep -v "nForce" | wc -l)
    echo $nv_count
}

_amd_counts()
{
    amd_count=$(_pci_info | grep -E "VGA|3D controller" | grep "Advanced Micro Devices" | egrep -v "Kaveri|BeaverCreek|Sumo|Wrestler|Kabini|Mullins|Temash|Trinity|Richland|Stoney|Carrizo|Raven" | wc -l)
    echo $amd_count
}

_platform(){
    local platform
    [[ ! -z $platform ]] && echo $platform && return 0

    if [[ $(_nvidia_counts) -gt $(_amd_counts) ]]; then
        platform=nvidia
    else
        platform=amd
    fi
    echo $platform
}


_cmdline_value()
{
  cat /proc/cmdline | egrep -o "(^|[[:space:]])$1=[^[:space:]]+" | tr -d " " | cut -d "=" -f 2- | tail -n 1
}


_gpu_devices(){
    local plat devices
    plat="$1"
    if [ "$plat" == "nvidia" ]; then
        name="Graphics Device"
        devices=$(_pci_info | grep  -E "VGA|3D controller" | grep "NVIDIA" | grep -v "nForce" | awk '{print $1}')
    else
        name=Ellesmere
        devices=$(_pci_info | grep  -E "VGA|3D controller" | grep "Advanced Micro Devices" | egrep -v "Kaveri|BeaverCreek|Sumo|Wrestler|Kabini|Mullins|Temash|Trinity|Richland|Stoney|Carrizo|Raven" | awk '{print $1}')
    fi

    # GPUBLACKLIST="000.00.1,000.00.2"
    # if [ -n "$GPUBLACKLIST" ]; then
    #   export IFS=","
    #   for gpupci in $GPUBLACKLIST; do
    #         if ! echo $devices | grep -q $gpupci;then
    #             devices="${devices}\n${gpupci}"
    #         fi
    #   done
    # fi
    printf "$devices" | sort
}

_load_driver()
{
    local platform nodriver
    grep -q nodriver /proc/cmdline && return

    # if [ ! -z "$dev_ctrl" ] && [ "null" != "$dev_ctrl" ]  && [ -f _device_info ];then
    #     nodriver=$(echo $dev_ctrl | jq ".nodriver" | sed 's/null//')
    #     if [ ! -z "$nodriver" ] && [ $nodriver -eq 1 ];then
    #         return
    #     fi
    # fi
    platform=$1
    if [ "$platform" == "amd" ];then
        lsmod | grep -q amdgpu || modprobe amdgpu
    fi
    if [ "$platform" == "nvidia" ];then
        lsmod | grep -q nvidia || modprobe nvidia
    fi
}

_device_info()
{
    local info platform jsonSTR disable one idx

    [[ -s $device_info ]] && info=$(cat $device_info) && echo $info && return 0

    if [[ $(_nvidia_counts) -gt $(_amd_counts) ]]; then
        platform=nvidia
    else
        platform=amd
    fi

    devices=$(_gpu_devices $platform)
    if [ ! -z "$devices" ];then
        jsonSTR='{}'
        idx=0
        for busid in $devices; do

            disable=0
            if [ ! -z "$dev_ctrl" ] && [ "null" != "$dev_ctrl" ] && [ -f _device_info ];then
                cat $dev_ctrl | jq ".gpu_control | .[] | select(.Disable==1) | .BusID" | grep -q "$busid" && disable=1
            fi
            one=$(jq -n -c \
            --arg name "$name" --arg bus_id $busid \
            --arg disable $disable --arg gpuid $idx \
            '{"name": $name, "bus_id": $bus_id, "disable": ($disable|tonumber), "gpuid": ($gpuid|tonumber), "physic_id": ($gpuid|tonumber)}')
            tmp=$(echo $jsonSTR | jq -c --argjson onegpu "$one" '.'\"${busid}\"'=$onegpu')
            jsonSTR=$tmp

            ((idx++))
        done
        [[ $jsonSTR != '{}' &&  $jsonSTR != '' ]] && echo $jsonSTR > $device_info && echo $jsonSTR
    fi
}

_prettytable_format(){
    python3 - "$1" <<END
import sys
from prettytable import from_csv
with open(sys.argv[1], "r") as fp:
    x = from_csv(fp)
print(x)
END
}

_format_status(){
    allinfo=false
    [[ "-a" == "$1" ]] && allinfo=true
    tmpf="/tmp/.$$"

    test -f $tmpf && unlink $tmpf

    local extra="" extra_data=""
    $allinfo && extra='"MODEL", "VBIOS",'

    echo '"GPUID", "BUSID", "NAME", '$extra' "MSIZE", "CORE", "MEM", "POWER", "FAN", "TEMP"' > $tmpf
    while read -r gpu
    do
        extra_data=""
        $allinfo && extra_data='+.memory_model+","+.vbios_version+","'

        echo $gpu | jq -r -c '((.gpuid|tostring)+","+.bus_id+","+.name+","'$extra_data'+(.max_mem_size|tostring)+","+(.core_clock|tostring)+","+(.mem_clock|tostring)+","+(.power_usage|tostring)+","+(.fan_percent|tostring)+","+(.temperature|tostring))' >> $tmpf

    done <<< $(cat $gpu_status_for_agent|jq -c ".[]")
    _prettytable_format $tmpf
    test -f $tmpf && unlink $tmpf
}


_wait_xorg()
{
    [[ $(_platform) == "amd" ]] && return 0
    for i in `seq 0 4`;do
        xpid=$(ps aux | grep '/usr/lib/Xorg' | grep -v grep | wc -l)
        [[ $xpid -gt 0 ]] && return 0
        _log "wait x before overclock: $i"
        sleep 2
    done
    return 1
}

_report_gpu_info(){
    return 0
}
