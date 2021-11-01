

version=$(echo $overinfo | jq '.oc_info.version')
_get_value()
{
    # pname=$1
    # gpuname=$2
    # if [ $version -eq 3 ];then
    #     echo $overinfo | jq -r '.oc_info.gpus."'"$gpuname"'" | .default | .'$pname | head -n1 | sed 's/null//' 2>/dev/null
    #     return
    # fi
    # if [ $version -eq 4 ];then
    #     echo $overinfo | jq -r '.oc_info.gpu.'$(_platform)'.'$pname' | .[] | select(.model=="'"$gpuname"'") | .default' | head -n1 | sed 's/null//' 2>/dev/null
    # fi

    
    local ret pname gpuname fixgpuname
    pname=$1
    gpuname=$2
    fixgpuname=$(echo $gpuname | sed 's/Vega /Vega/')

    if [ $version -eq 3 ];then
        ret=$(echo $overinfo | jq -r '.oc_info.gpus."'"$gpuname"'" | .default | .'$pname 2>/dev/null | head -n1 | sed 's/null//')
        [[ ! -z "$ret" ]] && echo $ret && return 
        ret=$(echo $overinfo | jq -r '.oc_info.gpus."'"$fixgpuname"'" | .default | .'$pname 2>/dev/null | head -n1 | sed 's/null//')
        [[ ! -z "$ret" ]] && echo $ret && return 
        ret=$(echo $overinfo | jq -r '.oc_info.gpus."'"$fixgpuname"'" | .'$algorithm' | .'$pname 2>/dev/null | head -n1 | sed 's/null//')
        [[ ! -z "$ret" ]] && echo $ret && return 
        
    fi
    if [ $version -eq 4 ];then
        ret=$(echo $overinfo | jq -r '.oc_info.gpu.'$(_platform)'.'$pname' | .[] | select(.model=="'"$gpuname"'") | .default' 2>/dev/null | head -n1 | sed 's/null//' )
        [[ ! -z "$ret" ]] && echo $ret && return 
        ret=$(echo $overinfo | jq -r '.oc_info.gpu.'$(_platform)'.'$pname' | .[] | select(.model=="'"$fixgpuname"'") | .default' 2>/dev/null | head -n1 | sed 's/null//')
        [[ ! -z "$ret" ]] && echo $ret && return 
    fi
}


_get_fan(){
    [[ $version -eq 4 ]] && fan_speed=$(echo $overinfo | jq '.oc_info.gpu.amd.GPUTargetFanSpeed | .[]|.default' 2>/dev/null)
    [[ $version -eq 3 ]] && fan_speed=$(echo $overinfo | jq '.oc_info.fan.GPUTargetFanSpeed | .[4]' 2>/dev/null)
    echo $fan_speed
}