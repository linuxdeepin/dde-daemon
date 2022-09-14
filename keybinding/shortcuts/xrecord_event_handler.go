// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package shortcuts

import (
	"strings"

	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/util/keysyms"
)

type XRecordEventHandler struct {
	keySymbols           *keysyms.KeySymbols
	pressedMods          uint16
	historyPressedMods   uint16
	nonModKeyPressed     bool
	modKeyReleasedCb     func(uint8, uint16)
	allModKeysReleasedCb func()
}

func NewXRecordEventHandler(keySymbols *keysyms.KeySymbols) *XRecordEventHandler {
	return &XRecordEventHandler{
		keySymbols: keySymbols,
	}
}

//func (h *XRecordEventHandler) logPressedMods(title string) {
//	logger.Debug(title, "pressedMods:", Modifiers(h.pressedMods))
//}

func (h *XRecordEventHandler) handleButtonEvent(pressed bool) {
	if h.pressedMods > 0 {
		h.nonModKeyPressed = true
	}
}

func (h *XRecordEventHandler) handleKeyEvent(pressed bool, keycode uint8, state uint16) {
	keystr, _ := h.keySymbols.LookupString(x.Keycode(keycode), state)
	//var pr string
	//if pressed {
	//	pr = "PRESS"
	//} else {
	//	pr = "RELEASE"
	//}
	//logger.Debugf("%s keycode: [%d|%s], state: %v\n", pr, keycode, keystr, Modifiers(state))

	if pressed {
		mod, ok := key2Mod(keystr)
		if ok {
			h.pressedMods |= mod
			h.historyPressedMods |= mod
		} else {
			//logger.Debug("non-mod key pressed")
			if h.pressedMods > 0 {
				h.nonModKeyPressed = true
			}
		}
		//h.logPressedMods("pressed")

	} else {
		// release
		//h.logPressedMods("before release")
		mod, ok := key2Mod(keystr)
		if !ok {
			return
		}
		if h.pressedMods == h.historyPressedMods && !h.nonModKeyPressed {
			if h.modKeyReleasedCb != nil {
				logger.Debugf("modKeyReleased keycode %d historyPressedMods: %s",
					keycode, Modifiers(h.historyPressedMods))
				h.modKeyReleasedCb(keycode, h.historyPressedMods)
			}
		}
		h.pressedMods &^= mod
		//h.logPressedMods("after release")

		if h.pressedMods == 0 {
			h.historyPressedMods = 0
			h.nonModKeyPressed = false
			if h.allModKeysReleasedCb != nil {
				//logger.Debug("allModKeysReleased")
				h.allModKeysReleasedCb()
			}
		}
	}
}

func key2Mod(key string) (uint16, bool) {
	key = strings.ToLower(key)
	// caps_lock and num_lock
	if key == "caps_lock" {
		return keysyms.ModMaskCapsLock, true
	} else if key == "num_lock" {
		return keysyms.ModMaskNumLock, true
	}

	// control/alt/meta/shift/super _ l/r
	i := strings.IndexByte(key, '_')
	if i == -1 {
		return 0, false
	}

	match := key[0:i]
	switch match {
	case "shift":
		return keysyms.ModMaskShift, true
	case "control":
		return keysyms.ModMaskControl, true
	case "super":
		return keysyms.ModMaskSuper, true
	case "alt", "meta":
		return keysyms.ModMaskAlt, true
	}
	return 0, false
}
