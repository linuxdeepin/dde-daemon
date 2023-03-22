#!/bin/bash

count=$(ps -ef | grep deepin-camera | grep -w -c $USER)
echo $count
if [ $count -ge 2 ]
then
	echo "open deepin-camera"
	killall deepin-camera
else
	echo "close deepin-camera"
	dbus-send --session --print-reply --dest=com.deepin.SessionManager /com/deepin/StartManager com.deepin.StartManager.Launch string:/usr/share/applications/deepin-camera.desktop
fi

