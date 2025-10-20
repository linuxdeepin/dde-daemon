#!/bin/bash

# 如果没有传递参数，使用默认值
newSinkName="remap-sink-mono"
sinkMaster=""
channels="1"
channel_map="mono"

# 解析参数
while [[ $# -gt 0 ]]; do
    case $1 in
        --sink_master=*)
            sinkMaster="${1#*=}"
            shift
            ;;
        --channels=*)
            channels="${1#*=}"
            shift
            ;;
        --sink_name=*)
            newSinkName="${1#*=}"
            shift
            ;;
        *)
            shift
            ;;
    esac
done

moduleArgs="sink_name=$newSinkName"
if [ -n "$sinkMaster" ]; then
    moduleArgs="$moduleArgs sink_master=$sinkMaster"
fi

# Reload "module-remap-sink"
echo Reload \"module-remap-sink\" with \"channels=$channels\" $moduleArgs
pactl unload-module module-remap-sink 2>/dev/null

if pactl load-module module-remap-sink channels=$channels channel_map=$channel_map $moduleArgs; then
    pactl set-default-sink $newSinkName
fi
