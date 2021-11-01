_amdmeminfo()
{
    # [[ -s $gpu_info_amd_file ]] && str=$(cat $gpu_info_amd_file) && printf "$str" && return 0

    amdmeminfo -q -s -n > $gpu_info_amd_file 2
    if [ $? -ne 0 ];then
        _log "amdmeminfo error"
        unlink $gpu_info_amd_file
        return
    fi
    
    [[ -s $gpu_info_amd_file ]] && str=$(cat $gpu_info_amd_file) && printf "$str" | sed 's/Sienna_cichlid/Navi21/'

}

_gpuinfo_info()
{
    local gpuinfo_json_str idx str

    str="{}"
    [[ -s $gpu_info_static ]] && str=$(cat $gpu_info_static)
    
    if ! _driver_add_try $gpu_try_counts; then
        echo $str
        return 0
    fi

    gpuinfo_json_str="{}"
    idx=0

    amd_meminfo=$(_amdmeminfo)

    dev_info=$(_device_info)
   
    for busid in $(_gpu_devices)
    do
        [[ -z "$busid" ]] && continue

        local onegpu pci_id match_pci name vbios memvendor memtype asictype mem cardno

        pci_id=$(echo $busid | awk -F":" '{print $2}')
        match_pci=:${pci_id}.00.0:
        meminfo=$(grep "$match_pci" <<< "$amd_meminfo")

        if [[ -z $meminfo ]];then
            onegpu=$(echo $dev_info | jq -c '.'\"$busid\")
        else

            _IFS=$IFS && IFS=":"
            read -a meminfo <<< "$meminfo"
            IFS=$_IFS

            name=${meminfo[2]}
            vbios=${meminfo[3]}
            memvendor=${meminfo[4]}
            memtype=${meminfo[5]}
            asictype=${meminfo[6]}

            mem=$(echo $str | jq -c '.'\"$busid\".max_mem_size 2>/dev/null | head -n1 | sed 's/null/0/')
            cardno=$(echo $str | jq -c '.'\"$busid\".cardno 2>/dev/null | head -n1 | sed 's/null/-1/')
            if [ $mem -eq 0 ];then
                [[ `echo /sys/bus/pci/devices/$busid/drm/card*/` =~ \/card([0-9]+)\/ ]]
                cardno=${BASH_REMATCH[1]}
                if [[ -z $cardno ]]; then
                        echo "ERROR: can not match card id for GPU $busid" >&2
                        cardno=-1
                        sleep 1
                else
                    if [[ -e /sys/class/drm/card$cardno/device/mem_info_vram_total ]]; then
                            mem="`cat /sys/class/drm/card$cardno/device/mem_info_vram_total 2>/dev/null`" && mem="$(( $mem/1024/1024 ))" || mem=
                    elif [[ -e /sys/kernel/debug/dri/$cardno/amdgpu_vram ]]; then
                            mem="`stat -c %s /sys/kernel/debug/dri/$cardno/amdgpu_vram 2>/dev/null`" && mem="$(( $mem/1024/1024 ))" || mem=
                    fi
                    [[ $memvendor == Unknown* ]] && memvendor=$(cat /sys/class/drm/card${cardno}/device/mem_info_vram_vendor || echo "Unknown")
                fi
            else
                ((idx++))
            fi

            onegpu=$(echo $dev_info | jq -c --arg name "$name" --arg vbios "$vbios" --arg memvendor "$memvendor" \
            --arg memtype "$memtype" --arg asictype "$asictype" --arg memsize $mem \
            --arg cardno "$cardno" \
            '.'\"$busid\"' | .cardno=($cardno|tonumber) | .name=$name | .memory_model=$memvendor | .memory_type=$memtype | .vbios_version=$vbios | .max_mem_size=($memsize|tonumber) | .platform="amd" | .asic_type=$asictype')

        fi

        if [ ! -z "$onegpu" ]; then
            tmp=$(echo $gpuinfo_json_str | jq -c --argjson onegpu "$onegpu" '.'\"${busid}\"'=$onegpu')
            gpuinfo_json_str=$tmp
        fi
    done
   
    [[ "$gpuinfo_json_str" != "{}" && "$gpuinfo_json_str" != "" ]] && echo $gpuinfo_json_str > $gpu_info_static && echo  $gpuinfo_json_str && return 0
    [[ "$gpuinfo_json_str" == "{}" ]] && echo $dev_info > $gpu_info_static && echo $dev_info

}


_gpuinfo_status()
{

    jsonGPUSTR="{}"

    local corev core_mhz mem_mhz mtemp pwr fan memv

    while read -r gpu;do

        busid=$(echo $gpu | jq -cr .bus_id)
        local asic_type
        asic_type=$(echo $gpu | jq -cr .asic_type)
        # cardno=$(echo $gpu | jq -cr .cardno)
        [[ `echo /sys/bus/pci/devices/$busid/drm/card*/` =~ \/card([0-9]+)\/ ]]
        cardno=${BASH_REMATCH[1]}
        if [ -z "$cardno" ];then
            corev=0
            core_mhz=0
            mem_mhz=0
            mtemp=0
            mtemp=0
            pwr=0
            fan=0
            memv=0
        else

            fanrpm=`cat /sys/class/drm/card$cardno/device/hwmon/hwmon*/pwm1 2>/dev/null` #rpm from fan
            max_fanrpm=`cat /sys/class/drm/card$cardno/device/hwmon/hwmon*/pwm1_max 2>/dev/null` #rpm from fan

            sclk=`grep -m1 '*' /sys/class/drm/card$cardno/device/pp_dpm_sclk 2>/dev/null`
            core_dpm=`echo $sclk | awk '{print $1}' | sed -e 's/://'`

            ppod=`cat /sys/class/drm/card$cardno/device/pp_od_clk_voltage 2>/dev/null`

            [[ ! -z "$ppod" ]] && [[ ! -z "$asic_type" ]] && [[ ! "$asic_type" =~ "Navi" ]] &&
                corev=`echo "$ppod" | grep -m1 -A8 "OD_SCLK" | grep -m1 $core_dpm | awk '{print $3}' | sed 's/mV//'`

            unset asic_type

            memv=`cat /sys/kernel/debug/dri/$cardno/amdgpu_pm_info | grep -m1 '(VDDGFX)' | awk '{print $1}'`
            core_mhz=`cat /sys/kernel/debug/dri/$cardno/amdgpu_pm_info | grep -m1 '(SCLK)' | awk '{print $1}'`
            mem_mhz=`cat /sys/kernel/debug/dri/$cardno/amdgpu_pm_info | grep -m1 '(MCLK)' | awk '{print $1}'`
            mtemp=`cat /sys/class/drm/card$cardno/device/hwmon/hwmon*/temp1_input 2>/dev/null`
            [[ -e /sys/kernel/debug/dri/$cardno/amdgpu_pm_info ]] &&
                pwr=`cat /sys/kernel/debug/dri/$cardno/amdgpu_pm_info | grep -m1 '(average GPU)' | awk '{print $1}'`
            pwr=${pwr%.*}
        fi

        [[ -z $corev ]] && corev=0
        [[ -z $core_mhz ]] && core_mhz=0
        [[ -z $mem_mhz ]] && mem_mhz=0
        [[ -z $mtemp ]] && mtemp=0
        [[ -z $mtemp ]] && mtemp=0
        [[ -z $pwr ]] && pwr=0
        [[ -z $memv ]] && memv=0

        [[ $fanrpm -gt $max_fanrpm ]] && fan=100
        [[ -z $fanrpm ]] && fanrpm=0
        [[ $fanrpm -lt $max_fanrpm ]] && fan=$(($fanrpm*100/$max_fanrpm))
        [[ -z $fan ]] && fan=0

        onegpu=$(echo $gpu | jq --arg fan "$fan" --arg temp "$(($mtemp/1000))" --arg power "$pwr" \
                --arg core_mhz "$core_mhz" --arg mem_mhz "$mem_mhz" --arg memv $memv --arg corev $corev \
                    ' .fan_percent=($fan|tonumber) | .temperature=($temp|tonumber) | .core_clock=($core_mhz|tonumber) | .mem_clock=($mem_mhz|tonumber) | .power_usage=($power|tonumber) | .mem_voltage=($memv|tonumber) | .sclk_voltage=($corev|tonumber)')

        if [ ! -z "$onegpu" ]; then
            tmp=$(echo $jsonGPUSTR | jq -c --argjson onegpu "$onegpu" '.'\"${busid}\"'=$onegpu')
            jsonGPUSTR=$tmp
        fi
    done <<< "$(_gpuinfo_info | jq -cr '.[]')"

    [[ "$jsonGPUSTR" != "" ]] && echo $jsonGPUSTR
}
