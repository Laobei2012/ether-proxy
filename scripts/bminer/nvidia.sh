_nvinfo()
{
    _load_driver nvidia

    [[ -s $gpu_info_nv_file ]] && str=$(cat $gpu_info_nv_file) && printf "$str" && return 0

    nvidia-smi --format=csv,noheader,nounits --query-gpu=gpu_bus_id,name,memory.total,vbios_version,power.default_limit,power.min_limit | sed 's/\[N\/A\]/0/g' > $gpu_info_nv_file
    if [ $? -ne 0 ];then
        _log "nvinfo error"
        unlink $gpu_info_nv_file
        return
    fi
    
    [[ -s $gpu_info_nv_file ]] && str=$(cat $gpu_info_nv_file) &&  printf "$str"

}

_gpuinfo_info()
{

    str="{}"
    [[ -s $gpu_info_static ]] && str=$(cat $gpu_info_static)
    
    if ! _driver_add_try $gpu_try_counts; then
        echo $str
        return 0
    fi
    
    gpuinfo_json_str="{}"
    idx=0
   

    for busid in $(_gpu_devices nvidia)
    do
        [[ -z "$busid" ]] && continue

        nvinfo=$(_nvinfo | grep -i "$busid")
        local onegpu
        if [[ -z $nvinfo ]];then
            onegpu=$(_device_info | jq -c '.'\"$busid\")
        else

            name=`awk -F', ' '{print $2}' <<< $nvinfo`
            memsize=`awk -F', ' '{print $3}' <<< $nvinfo`
            vbios=`awk -F', ' '{print $4}' <<< $nvinfo`
            df_pow=`awk -F', ' '{print $5}' <<< $nvinfo`
            df_pow=${df_pow%.*}
            min_pow=`awk -F', ' '{print $6}' <<< $nvinfo`

            onegpu=$(echo $(_device_info) | jq -c --arg name "$name" --arg vbios "$vbios" --arg memsize "$memsize" \
            --arg df_pow "$df_pow" --arg min_pow "$min_pow" \
            '.'\"$busid\"' | .name=$name | .vbios_version=$vbios | .max_mem_size=($memsize|tonumber) | .platform="nvidia" | .default_power_limit=($df_pow|tonumber) | .min_power_limit=($min_pow|tonumber)')

        fi

        tmp=$(echo $gpuinfo_json_str | jq --argjson onegpu "$onegpu" '.'\"${busid}\"'=$onegpu')
        gpuinfo_json_str=$tmp

        ((idx++))
    done

    [[ "$gpuinfo_json_str" != "{}" && "$gpuinfo_json_str" != "" ]] && echo $gpuinfo_json_str > $gpu_info_static && echo  $gpuinfo_json_str && return 0
    [[ "$gpuinfo_json_str" == "{}" ]] && _device_info > $gpu_info_static && echo $(_device_info)

}

_gpuinfo_status(){
    
    local nv_status one_info jsonGPUSTR
    jsonGPUSTR='{}'
    nv_status=$(nvidia-smi --format=csv,noheader,nounits --query-gpu=gpu_bus_id,temperature.gpu,fan.speed,power.draw,power.limit,clocks.mem,clocks.sm | sed 's/\[N\/A\]/0/g')
    while read gpu;do 
        busid=$(echo $gpu | jq -cr .bus_id)
        one_info=$(grep -i "$busid"<<<$nv_status)
        local onegpu temp fan power pl mem_clock core_clock
        if [[ -z $one_info ]];then
            onegpu="$gpu"
        else
            temp=`awk -F', ' '{print $2}' <<< $one_info`
            fan=`awk -F', ' '{print $3}' <<< $one_info`
            power=`awk -F', ' '{print $4}' <<< $one_info`
            pl=`awk -F', ' '{print $5}' <<< $one_info`
            mem_clock=`awk -F', ' '{print $6}' <<< $one_info`
            core_clock=`awk -F', ' '{print $7}' <<< $one_info`

            re='^[+-]?[0-9]+([.][0-9]+)?$'
            [[ -z "$temp" || ! $temp =~ $re ]] && temp=-1
            [[ -z "$fan" || ! $fan =~ $re ]] && fan=-1
            [[ -z "$power" || ! $power =~ $re ]] && power=-1
            [[ -z "$pl" || ! $pl =~ $re ]] && pl=-1

            [[ -z "$mem_clock" || ! $mem_clock =~ $re ]] && mem_clock=-1
            [[ -z "$core_clock" || ! $core_clock =~ $re ]] && core_clock=-1

            pl=${pl%.*}
            power=${power%.*}

            onegpu=$(echo $gpu | jq -c --arg temp "$temp" --arg fan "$fan" --arg power "$power" \
            --arg pl "$pl" --arg mem_clock "$mem_clock" --arg core_clock "$core_clock" \
            '.temperature=($temp|tonumber) | .fan_percent=($fan|tonumber) | .power_limit=($power|tonumber) | .power_limit=($pl|tonumber) | .power_usage=($power|tonumber) | .mem_clock=($mem_clock|tonumber) | .core_clock=($core_clock|tonumber)')
        fi
        tmp=$(echo $jsonGPUSTR | jq -c --argjson onegpu "$onegpu" '.'\"${busid}\"'=$onegpu')
        jsonGPUSTR=$tmp

    done <<< $(_gpuinfo_info | jq -c '.[]')
    [[ "$jsonGPUSTR" != "{}" ]] && echo $jsonGPUSTR
}
