#!/bin/sh
case $1/$2 in
        pre/*)
        ;;
        post/*)
            gdbus call -y -d org.deepin.system.Power1 -o /org/deepin/system/Power1 -m org.deepin.system.Power1.Refresh --timeout 2
        ;;
esac
