
# _log(){
#     echo "`date \"+%Y-%m-%d %H:%M:%S\"` $*" >> ${BMINER_RUN_PATH}/bminer.log
# }


export LC_ALL=en_US.UTF-8
export LANG=en_US.UTF-8

overpath=${BMINER_DATA_PATH}/overinfo
overpath_old=${BMINER_DATA_PATH}/overinfo.old

# _log "overlock.sh runing ..."

_gpu_overclock(){

    action=$1
    if [ "reset" == "$action" ] ;then
        [[ -f $overpath_old ]] && unlink $overpath_old 
        return 0
    fi

    # 检测超频信息是否变更
    [[ "force" != "$action" ]] && [[ -f $overpath_old ]]  && cmp $overpath_old $overpath && return 0

    if [[ -f $overpath ]]; then
        overinfo=$(cat $overpath)
    else
        _log "E: $overpath not exist."
        return 1
    fi

    overtype=$(echo $overinfo | jq -r '.oc_type')
    algorithm=$(echo $overinfo | jq -r '.algorithm')

    [[ -z "$algorithm" ]] && algorithm=ethash
    [[ -z "$overtype" ]] && _log "E: parse over info" && return 1

    if [ "$overtype" == "oc_host" ];then
        . $bminer_bin_path/overhost.sh
    elif [ "$overtype" == "oc_tpl" ];then
        . $bminer_bin_path/overtpl.sh
    else
        _log "E: $overtype current not supported"
        return 2
    fi

    overed_counts=0
    while read -r onegpu
    do
        
        gpuid=$(echo $onegpu | jq '.gpuid')
        disable=$(echo $onegpu | jq '.disable')
        if [ $disable -eq 1 ];then
            continue
        fi
        gpuname=$(echo $onegpu | jq -r '.name')
        busid=$(echo $onegpu | jq -r '.bus_id')

        [[ "$overtype" == "oc_host" ]] && value_key="$busid"
        [[ "$overtype" == "oc_tpl" ]] && value_key="$gpuname"

        if [ -z "$value_key" ];then
            continue
        fi

        GPUClock=$(_get_value GPUClock "$value_key")

        core_clock=$(_get_value GPUGraphicsClockOffset "$value_key")
        mem_clock=$(_get_value GPUMemoryTransferRateOffset "$value_key")
        power_limit=$(_get_value PowerLimit "$value_key")
        vddc=$(_get_value VDDC "$value_key")
        vddci=$(_get_value VDDCI "$value_key")
        mvdd=$(_get_value MVDD "$value_key")
        ref=$(_get_value REF "$value_key")
        ref_txt=$(_get_value REF_TXT "$value_key")
        core_state=$(_get_value CORE_STATE "$value_key")
        mem_state=$(_get_value MEM_STATE "$value_key")
        level=$(echo $onegpu | jq -r '.mem_level')
        driver_path="/sys/bus/pci/devices/$busid"
        local params
        params=""
        
        if [ "$(_platform)" == "amd" ];then
            local logmsg=""
            
            for i in `seq 0 10`;do
                    test -f ${driver_path}/pp_table && break
                    _log "oc: driver not ready: $i"
                    sleep 1
            done

            asic_type=$(echo $onegpu | jq -r '.asic_type' | tr [:upper:] [:lower:])
            if [ "$asic_type" == "navi10" ] || [ "$asic_type" == "navi14" ] || [ "$asic_type" == "navi21" ] || [ "$asic_type" == "dimgrey_cavefish" ];then
                # GFX VDDC (Core), mV
                NAVI_CVDDC_MIN=600   # Min VDDC - Gfx Core
                NAVI_CVDDC_MAX=1200  # Max VDDC - Gfx Core
                NAVI_CVDDC_SAFE=950  # Default fail safe voltage
                # Memory Interface Controller Interface Voltage, mV
                NAVI_VDDCI_MIN=650   # Min VDDCI
                NAVI_VDDCI_MAX=850   # Max VDDCI
                # Memory Voltage, mV
                NAVI_MVDD_MIN=1200   # Min MVDD
                NAVI_MVDD_MAX=1350   # Max MVDD
                # Clocks, MHz
                NAVI_MaxMemClock=1075 # Max memory clock
                NAVI_SOC_VDD_MIN=650   # Min SoC VDD
                NAVI_SOC_VDD_MAX=1200  # Max SoC VDD
                
                local args=""
                if [[ ${vddci} -ge $NAVI_VDDCI_MIN && ${vddci} -le $NAVI_VDDCI_MAX ]]; then
                   vlt_vddci=$(($vddci*4))
                   args+="smc_pptable/MemVddciVoltage/1=${vlt_vddci} smc_pptable/MemVddciVoltage/2=${vlt_vddci} smc_pptable/MemVddciVoltage/3=${vlt_vddci} "
                   logmsg="vddci: $vddci, "
                fi
                if [[ ${mvdd} -ge $NAVI_MVDD_MIN && ${mvdd} -le $NAVI_MVDD_MAX ]]; then
                   vlt_mvdd=$((${mvdd} * 4 ))
                   args+="smc_pptable/MemMvddVoltage/1=${vlt_mvdd} smc_pptable/MemMvddVoltage/2=${vlt_mvdd} smc_pptable/MemMvddVoltage/3=${vlt_mvdd} "
                   logmsg+="mvdd: $mvdd, "
                fi
                if [ "$asic_type" == "navi21" ] || [ "$asic_type" == "dimgrey_cavefish" ];then
                    args+="smc_pptable/VcBtcEnabled=0 smc_pptable/dBtcGbGfxDfllModelSelect=2 smc_pptable/DpmDescriptor/0/VoltageMode=2 smc_pptable/MaxVoltageGfx=$((NAVI_CVDDC_SAFE*4)) "
                else
                    args+="overdrive_table/max/8=${NAVI_MaxMemClock} \
                overdrive_table/min/3=${NAVI_CVDDC_MIN} overdrive_table/min/5=${NAVI_CVDDC_MIN} overdrive_table/min/7=${NAVI_CVDDC_MIN}"
                fi
               
                upp -p ${driver_path}/pp_table set \
                    smc_pptable/FanStopTemp=0 smc_pptable/FanStartTemp=10 smc_pptable/FanZeroRpmEnable=0 smc_pptable/MinVoltageGfx=$(( NAVI_CVDDC_MIN * 4 )) \
                    $args --write >> ${BMINER_RUN_PATH}/bminer.log 2>&1

                if [[ ! -z "$core_clock" && $core_clock -gt 0 ]];then
                    echo "s 1 $core_clock" > ${driver_path}/pp_od_clk_voltage
                    if [ "$asic_type" != "navi21" ] && [ "$asic_type" != "dimgrey_cavefish" ];then
                        [[ $vddc -ge $((NAVI_CVDDC_MIN+25)) ]] && echo "vc 1 $(($core_clock-100)) $(($vddc-25))" > ${driver_path}/pp_od_clk_voltage
                        echo "vc 2 $core_clock $vddc" > ${driver_path}/pp_od_clk_voltage 
                    fi
                    logmsg+="vddc: $vddc, core_clock: $core_clock, "
                fi
                
                
                if [[ ! -z "$mem_clock" && $mem_clock -gt 0 ]];then
                    echo "m 1 $mem_clock" > ${driver_path}/pp_od_clk_voltage
                    logmsg+="mem_clock: $mem_clock, "
                fi
                echo "manual" > ${driver_path}/power_dpm_force_performance_level

                echo 5 > ${driver_path}/pp_power_profile_mode
                echo c > ${driver_path}/pp_od_clk_voltage

            elif [ "$asic_type" == "vega20" ];then
                # Actual overclocking
                # r - reverts powerplay
                echo 'r' > ${driver_path}/pp_od_clk_voltage 2>/dev/null
                # c - commit changes
                echo 'c' > ${driver_path}/pp_od_clk_voltage

                echo 'manual' > ${driver_path}/power_dpm_force_performance_level
                # sets compute mode in new kernels
                echo '5'      > ${driver_path}/pp_power_profile_mode

                # a bit above is increasing VBIOS max limit if needed (only for Navi10)
                if [[ $mem_clock -gt 0 ]]; then 
                    echo "m 1 ${mem_clock}" > ${driver_path}/pp_od_clk_voltage 2>/dev/null
                    logmsg+="mem_clock: $mem_clock, "
                fi

                if [[ $core_clock -gt 0 ]]; then
                    echo "s 1 ${core_clock}" > ${driver_path}/pp_od_clk_voltage 2>/dev/null
                    logmsg+="core_clock: $core_clock, "
                fi

                if [[ ${vddc} -gt 0 ]]; then
                    echo "vc 2 ${core_clock} ${vddc}" > ${driver_path}/pp_od_clk_voltage 2>/dev/null
                    logmsg+="vddc: $vddc, "
                fi
                # c - commit changes
                echo 'c' > ${driver_path}/pp_od_clk_voltage
            elif [ "$asic_type" == "vega10" ];then
                # echo "do nothing"
                . $bminer_bin_path/oc.vega10.sh

            else
                . $bminer_bin_path/oc.polaris.sh
            fi
            
            #ref='--rtp 6 --ref 7500 --rcdrd 13 --rp 13 --rrds 3 --faw 12 --ras 25 --rc 38'
            #trim a string
            ref=$(echo  "$ref" | xargs)

            [[ ! -z "$ref" ]] && [[ $ref == ?(-)+([0-9]) ]] && [[ $ref -gt 0 ]] && sref="--ref $ref"
            [[ ! -z "$ref_txt" ]] && sref=$(echo $ref_txt | sed -e "s/^\"//" -e "s/\"$//")
            
            if [[ ! -z "$sref" ]];then
                memtweak_info=${BMINER_RUN_PATH}/.amdmemtweak
                [[ -f $memtweak_info ]] || amdmemtweak --current > $memtweak_info
                tweakid=$(cat $memtweak_info | grep "^GPU " | grep "$busid" | awk '{print $2}' | sed 's/://')
                [[ ! -z "$tweakid" ]] && amdmemtweak --gpu $tweakid $sref && echo "tweak ok"
                logmsg+="ref: $sref"
            fi

            [[ ! -z "$logmsg" ]] && _log "$logmsg"
            ((overed_counts++))
            
        else
            _wait_xorg && ((overed_counts++))

            [[ ! -z $power_limit ]] && [[ $power_limit -ne -1 ]] && \
            _log "nvidia-smi -i $gpuid -pm 0 && nvidia-smi -i $gpuid -pl $power_limit" && \
            nvidia-smi -i $gpuid -pm 0 && nvidia-smi -i $gpuid -pl $power_limit  >> ${BMINER_RUN_PATH}/bminer.log 2>&1

            [[ ! -z $GPUClock ]] && [[ $GPUClock -ne -1 ]] && [[ $GPUClock -ne 0 ]] && \
            _log "nvidia-smi -i $gpuid -lgc $GPUClock" && \
            nvidia-smi -i $gpuid -lgc $GPUClock  >> ${BMINER_RUN_PATH}/bminer.log 2>&1

            level=$(nvidia-settings -c :1 -q [gpu:${gpuid}]/GPUPerfModes | grep -Poi "(?<=perf=)(\d+)"  | tail -n 1)
            [[ -z "$level" ]] && continue
            [[ ! -z "$core_clock" && $core_clock -ne -1 ]] && params+="-a [gpu:${gpuid}]/GPUGraphicsClockOffset[${level}]=$core_clock "
            [[ ! -z "$mem_clock" && $mem_clock -ne -1 ]] && params+="-a [gpu:${gpuid}]/GPUMemoryTransferRateOffset[${level}]=$mem_clock "
            [[ ! -z "$params" ]] && _log "nvidia-settings -c :1 $params" && nvidia-settings -c :1 $params  1>> ${BMINER_RUN_PATH}/bminer.log 2> /dev/null  
        fi
        
    done << EOF
        `_gpuinfo_info | jq -c '.[]'`
EOF
    gpu_counts=$(_gpuinfo_info | jq 'keys|length')
    [[ $overed_counts -ge $gpu_counts ]] && cp $overpath $overpath_old && return 0

}


_fan_control()
{
    [[ -f $overpath ]] && overinfo=$(cat $overpath)
    # [[ -z "$overinfo" ]] && _log "E: no over info"


    overtype=$(echo $overinfo | jq -r '.oc_type')

    # [[ -z "$overtype" ]] && _log "E: parse over info"

    if [ "$overtype" == "oc_host" ];then
        . $bminer_bin_path/overhost.sh
    elif [ "$overtype" == "oc_tpl" ];then
        . $bminer_bin_path/overtpl.sh
    else
        # _log "E: $overtype current not supported"
        return
    fi

    local onegpu gpuid busid driver_path fan_speed current_speed max idx

    platform=$(_platform)
    if [ $platform == "nvidia" ];then
        nvs=$(_gpuinfo_info | jq 'keys|length')
        if [ ! -f ${BMINER_RUN_PATH}/.fancounts ];then
            nvfs=$(nvidia-settings -q GPUTargetFanSpeed -c :1  2>/dev/null | grep Attribute )
            if [[ $? -eq 0 ]];then
                wc -l <<< $nvfs > ${BMINER_RUN_PATH}/.fancounts
            fi
        fi
        fans=$(cat ${BMINER_RUN_PATH}/.fancounts)
        [[ $fans -eq 0 ]] && return
    fi

    idx=0
    while read -r onegpu
    do
        busid=$(echo $onegpu | jq -r '.bus_id')
        [[ $busid == "null" ]] && continue

        current_speed=$(echo $onegpu | jq '.fan_percent')
        fan_speed=$(_get_fan "$busid")
        [[ -z "$fan_speed" ]] && fan_speed=80

        if [[ ${fan_speed} -le 0 ]];then
            ((idx++)) 
            continue
        fi

        if [[ ${current_speed} -ge $((fan_speed-5)) && ${current_speed} -le $((fan_speed+5)) ]];then
            ((idx++)) 
            continue
        fi
        
        if [[ $platform == "amd" ]];then
            driver_path="/sys/bus/pci/devices/$busid"
            pwm1_enable=`ls ${driver_path}/hwmon/hwmon*/pwm1_enable`
            if [[ $fan_speed -gt 0 ]] && [[ ! -z $pwm1_enable ]];then
                hwmondir=$(dirname $pwm1_enable)
                echo 1 > $hwmondir/pwm1_enable
                max=$(cat $hwmondir/pwm1_max)
                target_speed=$((max*fan_speed/100))
                echo $target_speed > $hwmondir/pwm1
                [[ $? -eq 0 ]] && _log "set fan to $busid with $target_speed"
            fi
        else
            if [ $fans -gt $nvs ];then
                number1=$((2*idx))
                number2=$((number1+1))
                /usr/bin/nvidia-settings -c :1 -a [gpu:$idx]/GPUFanControlState=1 -a [fan:$number1]/GPUTargetFanSpeed=$fan_speed -a [fan:$number2]/GPUTargetFanSpeed=$fan_speed 2>/dev/null 
            else
                /usr/bin/nvidia-settings -c :1 -a [gpu:$idx]/GPUFanControlState=1 -a [fan:$idx]/GPUTargetFanSpeed=$fan_speed 2>/dev/null 
            fi
        fi
        ((idx++))
    done << EOF
        `cat $gpu_status_for_agent | jq -c '.[]'`
EOF

}

