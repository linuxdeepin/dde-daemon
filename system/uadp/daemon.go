package uadp

import (
	"pkg.deepin.io/dde/daemon/loader"
	"pkg.deepin.io/lib/log"
)

type Daemon struct {
	*loader.ModuleBase
	manager *Manager
}

var (
	logger   = log.NewLogger("daemon/uadp")
	_manager *Manager
)

func NewDaemon(logger *log.Logger) *Daemon {
	daemon := new(Daemon)
	daemon.ModuleBase = loader.NewModuleBase("uadp", daemon, logger)
	return daemon
}

func (*Daemon) GetDependencies() []string {
	return []string{}
}

func init() {
	loader.Register(NewDaemon(logger))
}

func (d *Daemon) Start() error {
	if _manager != nil {
		return nil
	}

	var err error
	service := loader.GetService()
	_manager, err = NewManager(service)
	if err != nil {
		logger.Error("Failed to new uadp manager:", err)
		return err
	}

	err = service.Export(dbusPath, _manager)
	if err != nil {
		return err
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		return err
	}
	return nil
}

func (d *Daemon) Stop() error {
	if d.manager == nil {
		return nil
	}

	service := loader.GetService()
	err := service.ReleaseName(dbusServiceName)
	if err != nil {
		return err
	}

	err = service.StopExport(d.manager)
	if err != nil {
		return err
	}

	d.manager.destroy()
	d.manager = nil
	return nil
}
