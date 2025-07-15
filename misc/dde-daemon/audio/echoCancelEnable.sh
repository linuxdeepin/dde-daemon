#!/bin/bash

# 如果没有传递 aecArgs，使用默认值
newSourceName="echo-cancel-source"
aecArgs="analog_gain_control=0 digital_gain_control=1"
sourceMaster=""

# 解析参数
while [[ $# -gt 0 ]]; do
    case $1 in
        --aec_args=*)
            aecArgs="${1#*=}"
            shift
            ;;
        --source_master=*)
            sourceMaster="${1#*=}"
            shift
            ;;
        *)
            shift
            ;;
    esac
done

moduleArgs="source_name=$newSourceName"
if [ -n "$sourceMaster" ]; then
    moduleArgs="$moduleArgs source_master=$sourceMaster"
fi

# Reload "module-echo-cancel"
echo Reload \"module-echo-cancel\" with \"aec_args=$aecArgs\" $moduleArgs
pactl unload-module module-echo-cancel 2>/dev/null

if pactl load-module module-echo-cancel use_master_format=1 aec_method=webrtc aec_args=\"$aecArgs\" $moduleArgs; then
    pactl set-default-source $newSourceName
fi