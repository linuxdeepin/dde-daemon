package airplane_mode

// RadioKey radio key is used to match radio key event
type RadioKey int

// key code
// RadioKey include/linux/rfkill.h
const (
	BluetoothRadioKey RadioKey = 237
	WlanRadioKey      RadioKey = 238
	WWanRadioKey      RadioKey = 246
	AllRadioKey       RadioKey = 247
)

// Ignore should ignore key here
func (key RadioKey) Ignore() bool {
	var ignore bool
	switch key {
	case BluetoothRadioKey, WlanRadioKey, AllRadioKey:
		ignore = false
	default:
		ignore = true
	}
	return ignore
}

func (key RadioKey) ToRadioType() RadioType {
	var typ RadioType
	switch key {
	case BluetoothRadioKey:
		typ = BluetoothRadioType
	case WlanRadioKey:
		typ = WlanRadioType
	case AllRadioKey:
		typ = AllRadioType
	}
	return typ
}

// RadioType radio key is use to match key event
type RadioType int

// RadioType include/linux/rfkill.h
const (
	AllRadioType RadioType = iota
	WlanRadioType
	BluetoothRadioType
)

// ToRfkillType convert to rfkill type
func (typ RadioType) ToRfkillType() uint8 {
	var ty uint8
	switch typ {
	case AllRadioType:
		ty = 0
	case WlanRadioType:
		ty = 1
	case BluetoothRadioType:
		ty = 2
	}
	return ty
}

// RadioAction radio action use rfkill to operate radio
type RadioAction int

const (
	NoneRadioAction RadioAction = iota
	BlockRadioAction
	UnblockRadioAction
	ListRadioAction
	MonitorRadioAction
)

// ToRfkillAction convert to rfkill action
func (action RadioAction) ToRfkillAction() uint8 {
	var ac uint8
	switch action {
	case UnblockRadioAction:
		ac = 0
	case BlockRadioAction:
		ac = 1
	}
	return ac
}

// String action
func (action RadioAction) String() string {
	var name string
	switch action {
	case BlockRadioAction:
		name = "block"
	case UnblockRadioAction:
		name = "unblock"
	case ListRadioAction:
		name = "list"
	case MonitorRadioAction:
		name = "event"
	}
	return name
}

// BaseRadioModule the interface operate and check radio module
type BaseRadioModule interface {
	// Type radio type
	Type() RadioType
	// Module module name
	Module() string
	// Len rfkill item len
	Len() int
	// Block block this module
	Block() error
	// Unblock unblock this module
	Unblock() error
	// IsBlocked check if this module is blocked
	IsBlocked() bool
}

// RfkillEvent rfkill event
type RfkillEvent struct {
	Index uint32
	Typ   uint8
	Op    uint8
	Soft  uint8
	Hard  uint8
}
