// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package power

//func (m *Manager) Debug(cmd string) *dbus.Error {
//	logger.Debug("Debug", cmd)
//	switch cmd {
//	case "init-batteries":
//		devices := m.gudevClient.QueryBySubsystem("power_supply")
//		for _, dev := range devices {
//			m.addAndExportBattery(dev)
//		}
//		logger.Debug("initBatteries done")
//		for _, dev := range devices {
//			dev.Unref()
//		}
//
//	case "remove-all-batteries":
//		var devices []*gudev.Device
//		m.batteriesMu.Lock()
//		for _, bat := range m.batteries {
//			devices = append(devices, bat.newDevice())
//		}
//		m.batteriesMu.Unlock()
//
//		for _, dev := range devices {
//			m.removeBattery(dev)
//			dev.Unref()
//		}
//
//	case "destroy":
//		m.destroy()
//
//	default:
//		logger.Warning("Command not support")
//	}
//	return nil
//}
//
//func (bat *Battery) Debug(cmd string) *dbus.Error {
//	dev := bat.newDevice()
//	if dev != nil {
//		defer dev.Unref()
//
//		switch cmd {
//		case "reset-update-interval1":
//			bat.resetUpdateInterval(1 * time.Second)
//		case "reset-update-interval3":
//			bat.resetUpdateInterval(3 * time.Second)
//		default:
//			logger.Info("Command no support")
//		}
//	}
//	return nil
//}
