package grub_gfx

import (
	"sync"

	"pkg.deepin.io/dde/daemon/loader"
	"pkg.deepin.io/lib/log"
)

const moduleName = "grub-gfx"

var logger = log.NewLogger(moduleName)

type module struct {
	*loader.ModuleBase
	wg sync.WaitGroup
}

func (m *module) WaitEnable() {
	m.wg.Wait()
}

func (*module) GetDependencies() []string {
	return nil
}

func (d *module) Start() error {
	defer d.wg.Done()
	detectChange()
	return nil
}

func (d *module) Stop() error {
	return nil
}

func newModule() *module {
	d := new(module)
	d.ModuleBase = loader.NewModuleBase(moduleName, d, logger)
	d.wg.Add(1)
	return d
}

func init() {
	loader.Register(newModule())
}
