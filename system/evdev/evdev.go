package evdev

import (
	"pkg.deepin.io/dde/daemon/loader"
	"pkg.deepin.io/lib/log"
)

var logger = log.NewLogger("daemon/evdev")

func init() {
	loader.Register(NewModule())
}

type Module struct {
	m *Manager
	*loader.ModuleBase
}

func (m Module) GetDependencies() []string {
	return nil
}

func (m Module) Start() error {
	if m.m != nil {
		return nil
	}
	var err error
	logger.Debug("evtest mode module start")
	service := loader.GetService()
	m.m = newManager(service)
	err = service.Export(dbusPath, m.m)
	if err != nil {
		return err
	}

	go m.m.runEvtest()
	err = service.RequestName(dbusServiceName)
	if err != nil {
		return err
	}
	return nil
}

func (m Module) Stop() error {
	return nil
}

func NewModule() *Module {
	m := &Module{}
	m.ModuleBase = loader.NewModuleBase("evtest", m, logger)
	return m
}
