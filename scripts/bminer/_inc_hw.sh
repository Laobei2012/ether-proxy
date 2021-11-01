

function getMACAddr()
{
    dev=$(/usr/bin/ip link|grep "state UP"|awk -F: '{print $2}')
    mac=$(/usr/bin/ip link show $dev | grep ether | awk '{print $2}')
    if [ -z "$mac" ];then
        mac=$(/usr/bin/ip link show $dev | grep -o -E '([[:xdigit:]]{1,2}:){5}[[:xdigit:]]{1,2}' )
    fi
    echo $mac
}

function getHostAddr(){
    ip=$(ip route get 1 | awk '{print $7;exit}')
    echo $ip
}

function getPublicAddr(){
    ip=$(curl ifconfig.co -s)
    echo $ip
}

function getCPUInfo(){
    cpus=$(lscpu|awk '{if ($1~"^CPU\(s\)") print $2}')
    model=$(lscpu|awk -F: '{if ($1~"^Model name") print $2}'|xargs)
    echo $model,$cpus
}

function getMEMInfo(){
    # unit kB
    MemTotal=$(cat /proc/meminfo|awk '{if ($1~"MemTotal") print $2}')
    MemAvailable=$(cat /proc/meminfo|awk '{if ($1~"^MemAvailable") print $2}')
    echo $MemTotal,$MemAvailable
}


function getDiskInfo(){
    # unit Byte
    disk=$(df|awk '{if ($1~"/dev/sd") print $1","$2","$3}')
    echo $disk
}

function getUptime(){
    # up 1 week, 22 hours, 32 minutes,2021-10-12 19:01:26
    echo $(uptime -p),$(uptime -s)
}

function getOSVersion(){
    # up 1 week, 22 hours, 32 minutes,2021-10-12 19:01:26
    echo $(uname -srv)
}

