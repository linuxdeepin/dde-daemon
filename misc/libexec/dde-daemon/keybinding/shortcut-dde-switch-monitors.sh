#!/bin/bash

if ! outputs=$(dbus-send --print-reply --dest=org.deepin.dde.Display1 /org/deepin/dde/Display1 org.deepin.dde.Display1.ListOutputNames | grep -oP 'string "\K[^"]+'); then
    exit 1
fi

if [ $(echo "$outputs" | wc -l) -gt 1 ]; then
    dbus-send --print-reply --dest=org.deepin.dde.Osd1 / org.deepin.dde.Osd1.ShowOSD string:SwitchMonitors
fi
