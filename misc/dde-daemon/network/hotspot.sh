if [ "$1" = "init" ]; then
	# 添加正确的静态路由
	route add -net 192.168.88.0 netmask 255.255.255.0 dev wlan0
elif [ "$1" = "open" ]; then
	# 关闭无线网卡
	ifconfig wlan0 down

	# 设置成路由器， 打开iptables的NAT功能
	/sbin/iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE

	# 启动平板热点
	/usr/lib/deepin-daemon/pad-hotspot
elif [ "$1" = "close" ]; then
	# 打开无线网卡
	ifconfig wlan0 up
fi
