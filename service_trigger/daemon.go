package service_trigger

import (
	"github.com/linuxdeepin/go-lib/log"
	"pkg.deepin.io/dde/daemon/loader"
)

func init() {
	loader.Register(NewDaemon())
}

var logger = log.NewLogger("daemon/" + moduleName)

const moduleName = "service-trigger"

type Daemon struct {
	*loader.ModuleBase
	manager *Manager
}

func NewDaemon() *Daemon {
	d := &Daemon{}
	d.ModuleBase = loader.NewModuleBase(moduleName, d, logger)
	return d
}

func (d *Daemon) Start() error {
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
