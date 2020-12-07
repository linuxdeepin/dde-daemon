package airplane_mode

import (
	"sync"

	"pkg.deepin.io/dde/daemon/loader"
	"pkg.deepin.io/lib/log"
)

var logger *log.Logger

func init() {
	logger = log.NewLogger("daemon/airplane_mode")
	loader.Register(NewModule())
}

type Module struct {
	m *Manager
	*loader.ModuleBase
	wg sync.WaitGroup
}

func (m *Module) GetDependencies() []string {
	return nil
}

func (m *Module) WaitEnable() {
	m.wg.Wait()
	return
}

func (m *Module) Start() error {
	defer m.wg.Done()
	if m.m != nil {
		return nil
	}
	logger.Debug("airplane mode module start")
	service := loader.GetService()
	m.m = newManager(service)
	err := service.Export(dbusPath, m.m)
	if err != nil {
		return err
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		return err
	}
	return nil
}

func (m *Module) Stop() error {
	return nil
}

func NewModule() *Module {
	m := &Module{}
	m.ModuleBase = loader.NewModuleBase("airplane_mode", m, logger)
	m.wg.Add(1)
	return m
}
