package bluetooth

import "github.com/godbus/dbus"

const (
	dbusServiceNameV20 = "com.deepin.daemon.Bluetooth"
	dbusPathV20        = "/com/deepin/daemon/Bluetooth"
	dbusInterfaceV20   = dbusServiceNameV20
)

type BluetoothV20 struct {
	bluetooth *Bluetooth
}

func NewBluetoothV20(bluetooth *Bluetooth) *BluetoothV20 {
	bluetoothV20 := &BluetoothV20{
		bluetooth:bluetooth,
	}

	return bluetoothV20
}

func (b *BluetoothV20) GetInterfaceName() string {
	return dbusInterfaceV20
}

func (b *BluetoothV20) GetAdapters() (adaptersJSON string, busErr *dbus.Error) {
	logger.Debug("GetAdapters V20")
	return b.bluetooth.GetAdapters()
}