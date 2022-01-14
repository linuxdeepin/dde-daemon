package airplane_mode

func getRadioModule(key RadioType) BaseRadioModule {
	var module BaseRadioModule
	switch key {
	case BluetoothRadioType:
		module = &BluetoothRadio{GeneralRadioModule{typ: BluetoothRadioType, name: "bluetooth"}}
	case WlanRadioType:
		module = &WlanRadio{GeneralRadioModule{typ: WlanRadioType, name: "wlan"}}
	case AllRadioType:
		module = &AllRadio{GeneralRadioModule{typ: AllRadioType, name: "all"}}
	default:
		panic("radio type not exist")
	}
	return module
}

type GeneralRadioModule struct {
	// radio type and radio module name
	typ  RadioType
	name string
}

func (Op *GeneralRadioModule) Type() RadioType {
	return Op.typ
}

func (Op *GeneralRadioModule) Module() string {
	return Op.name
}

func (Op *GeneralRadioModule) Len() int {
	state, err := rfkillState(Op.typ)
	if err != nil {
		logger.Warningf("check rfkill blocked state failed, err: %v", err)
		// if cant check rfkill state, count is 0
		state.count = 0
	}
	return state.count
}

func (Op *GeneralRadioModule) Block() error {
	return rfkillAction(Op.typ, BlockRadioAction)
}

func (Op *GeneralRadioModule) Unblock() error {
	return rfkillAction(Op.typ, UnblockRadioAction)
}

func (Op *GeneralRadioModule) IsBlocked() bool {
	// check radio block state
	state, err := rfkillState(Op.typ)
	if err != nil {
		logger.Warningf("check rfkill blocked state failed, err: %v", err)
		// if cant read state, at this time /dev/rfkill may not included,
		// type cant blocked by rfkill command, so it is unblocked
		state.blocked = false
	}
	return state.blocked
}

// BluetoothRadio bluetooth radio
type BluetoothRadio struct {
	GeneralRadioModule
}

// WlanRadio wlan radio
type WlanRadio struct {
	GeneralRadioModule
}

// AllRadio rfkill module
type AllRadio struct {
	GeneralRadioModule
}
