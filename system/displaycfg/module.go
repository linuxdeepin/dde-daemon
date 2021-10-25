package displaycfg

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
	logger.Debug("module displaycfg start")
	service := loader.GetService()

	d := newDisplayCfg(service)
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

func newDisplayCfgModule(logger *log.Logger) *module {
	m := new(module)
	m.ModuleBase = loader.NewModuleBase("displaycfg", m, logger)
	return m
}

var logger = log.NewLogger("daemon/displaycfg")

func init() {
	loader.Register(newDisplayCfgModule(logger))
}
