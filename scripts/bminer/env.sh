_sourced_="__env_sourced_$$__"

if [ -z "${!_sourced_}" ]; then
    eval "$_sourced_=1"
else
    return 0
fi

export BMINER_BIN_PATH="/opt/bminer"
export BMINER_RUN_PATH="/var/run/bminer"
export BMINER_DATA_PATH="/opt/data/bminer"

export BMINER_MINERAPP_PATH="/opt"