
version=$(echo $overinfo | jq '.oc_info.version')

_get_value()
{
    param=$1
    busid=$2
    # echo $overinfo | jq '.oc_info.gpu.'$algorithm' |.[]| select(.BusID=="'"$busid"'") | .'$param'' 2>/dev/null | sed 's/null//' 2>/dev/null
    echo $overinfo | jq '.oc_info.gpu.'$algorithm' |.[]| select(.BusID|test("'"$busid"'")) | .'$param'' 2>/dev/null | sed 's/null//' 2>/dev/null
}


_get_fan(){
    busid=$1
    echo $overinfo | jq '.oc_info.fan|.[]| select(.BusID|test("'"$busid"'"))|.GPUTargetFanSpeed' 2>/dev/null | sed 's/null//'
}