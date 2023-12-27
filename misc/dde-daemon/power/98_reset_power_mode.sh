#!/bin/bash

dde-dconfig --get -a org.deepin.dde.daemon --list|grep "org.deepin.dde.daemon.power"
if [ $? -ne 0 ];then
  if [ ! -f "/var/lib/dde-daemon/power/config.json" ]; then
    qdbus --literal --system com.deepin.system.Power /com/deepin/system/Power com.deepin.system.Power.SetMode "balance"
  fi
fi
