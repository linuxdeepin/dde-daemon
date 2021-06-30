package network

import (
	"time"

	dbus "github.com/godbus/dbus"
	"pkg.deepin.io/lib/dbusutil"
)

const (
	interfaceIPConflict  = "com.deepin.system.IPWatchD"
	objectPathIPConflict = "/com/deepin/system/IPWatchD"
	memberIPConflict     = "IPConflict"
	signalNameIPConflict = "IPConflictCheck"
)

func (m *Manager) initIPConflictManager() {
	err := dbusutil.NewMatchRuleBuilder().
		Type("signal").
		Interface(interfaceIPConflict).
		Member(memberIPConflict).Build().
		AddTo(m.sysSigLoop.Conn())
	if err != nil {
		logger.Warning(err)
	}

	sysSigLoop.AddHandler(&dbusutil.SignalRule{
		Name: interfaceIPConflict + "." + memberIPConflict,
	}, func(sig *dbus.Signal) {
		if len(sig.Body) == 3 {
			ip, ok := sig.Body[0].(string)
			if !ok {
				logger.Warning("ip must string type")
				return
			}
			macA, ok := sig.Body[1].(string)
			if !ok {
				logger.Warning("mac must string type")
				return
			}
			macB, ok := sig.Body[2].(string)
			if !ok {
				logger.Warning("mac must string type")
				return
			}
			if len(macA) != 0 && len(macB) != 0 {
				err := m.service.Emit(m, memberIPConflict, ip, macB)
				if err != nil {
					logger.Debug(err)
				}
			}
		}
	})
}

func (m *Manager) RequestIPConflictCheck(ip, ifc string) (string, *dbus.Error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return "", dbusutil.ToError(err)
	}

	err = conn.Emit(objectPathIPConflict, interfaceIPConflict+"."+signalNameIPConflict, ip, ifc)
	if err != nil {
		return "", dbusutil.ToError(err)
	}

	conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0,
		"type='signal',path='/com/deepin/system/IPWatchD',interface='com.deepin.system.IPWatchD'")

	c := make(chan *dbus.Signal, 10)
	conn.Signal(c)

	t := time.NewTimer(500 * time.Millisecond)
	for {
		select {
		case <-t.C:
			logger.Debug("wait ip conflict signal timeout")
			return "", nil
		case v := <-c:
			if v.Name == "com.deepin.system.IPWatchD.IPConflict" && len(v.Body) == 3 {
				if val, ok := v.Body[0].(string); ok && ip == val {
					ret, ok := v.Body[2].(string)
					if !ok {
						logger.Warning("mac must string type")
						continue
					}

					return ret, nil
				}
			}
		}
	}
}
