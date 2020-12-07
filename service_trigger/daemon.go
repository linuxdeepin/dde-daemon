package service_trigger

import (
	"sync"

	"pkg.deepin.io/dde/daemon/loader"
	"pkg.deepin.io/lib/log"
)

func init() {
	loader.Register(NewDaemon())
}

var logger = log.NewLogger("daemon/" + moduleName)

const moduleName = "service-trigger"

type Daemon struct {
	*loader.ModuleBase
	manager *Manager
	wg      sync.WaitGroup
}

func NewDaemon() *Daemon {
	d := &Daemon{}
	d.ModuleBase = loader.NewModuleBase(moduleName, d, logger)
	d.wg.Add(1)
	return d
}

func (d *Daemon) WaitEnable() {
	d.wg.Wait()
}

func (d *Daemon) Start() error {
	defer d.wg.Done()
	m := newManager()
	m.start()
	d.manager = m
	return nil
}

func (d *Daemon) Stop() error {
	if d.manager != nil {
		err := d.manager.stop()
		if err != nil {
			return err
		}
		d.manager = nil
	}
	return nil
}

func (*Daemon) GetDependencies() []string {
	return []string{}
}
