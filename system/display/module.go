package display

import (
	"pkg.deepin.io/dde/daemon/loader"
	"pkg.deepin.io/lib/log"
)

type module struct {
	*loader.ModuleBase
}

func (m *module) GetDependencies() []string {
	return nil
}

func (m *module) Start() error {
	logger.Debug("module display start")
	service := loader.GetService()

	d := newDisplay(service)
	err := service.Export(dbusPath, d)
	if err != nil {
		return err
	}
	err = service.RequestName(dbusServiceName)
	if err != nil {
		return err
	}

	return nil
}

func (m *module) Stop() error {
	return nil
}

func newDisplayModule(logger *log.Logger) *module {
	m := new(module)
	m.ModuleBase = loader.NewModuleBase("display", m, logger)
	return m
}

var logger = log.NewLogger("daemon/display")

func init() {
	loader.Register(newDisplayModule(logger))
}
