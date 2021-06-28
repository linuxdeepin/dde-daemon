#!/bin/sh

killall brcm_patchram_plus1

echo 0 > /sys/class/rfkill/rfkill0/state
sleep 2
echo 1 > /sys/class/rfkill/rfkill0/state
sleep 2

brcm_patchram_plus1 --enable_hci --no2bytes --use_baudrate_for_download  --tosleep  200000 --baudrate 1500000 --patchram  /system/etc/firmware/"BCM4356A2.hcd" /dev/ttyS0 &


handlebluetooth() {
	count=0
	while [ $count -lt 10 ]
	do
		hciconfig hci0 $1
		if [ $? -eq 0 ]; then
		  break
		fi

		count=$(($count+1))
		sleep 1
	done
}

handlebluetooth $1
