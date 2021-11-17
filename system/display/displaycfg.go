package display

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/godbus/dbus"
	"pkg.deepin.io/lib/dbusutil"
)

const (
	dbusServiceName   = "com.deepin.system.Display"
	dbusInterfaceName = dbusServiceName
	dbusPath          = "/com/deepin/system/Display"
	configFilePath    = "/var/lib/dde-daemon/display/config.json"
)

//go:generate dbusutil-gen em -type Display

type Display struct {
	service *dbusutil.Service
	cfg     *Config
	cfgMu   sync.Mutex
	signals *struct {
		ConfigUpdated struct {
			updateAt string
		}
	}
}

func newDisplay(service *dbusutil.Service) *Display {
	d := &Display{
		service: service,
	}
	cfg, err := loadConfig(configFilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warning(err)
		}
	}
	d.cfg = cfg
	return d
}

func (d *Display) GetInterfaceName() string {
	return dbusInterfaceName
}

func (d *Display) GetConfig() (cfgStr string, busErr *dbus.Error) {
	var err error
	cfgStr, err = d.getConfig()
	return cfgStr, dbusutil.ToError(err)
}

func (d *Display) getConfig() (string, error) {
	d.cfgMu.Lock()
	defer d.cfgMu.Unlock()

	data, err := json.Marshal(d.cfg)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (d *Display) SetConfig(cfgStr string) *dbus.Error {
	err := d.setConfig(cfgStr)
	return dbusutil.ToError(err)
}

func (d *Display) setConfig(cfgStr string) error {
	var cfg Config
	err := json.Unmarshal([]byte(cfgStr), &cfg)
	if err != nil {
		return err
	}

	d.cfgMu.Lock()
	defer d.cfgMu.Unlock()
	d.cfg = &cfg

	err = saveConfig(&cfg, configFilePath)
	if err != nil {
		return err
	}

	err = d.service.Emit(d, "ConfigUpdated", cfg.UpdateAt)
	if err != nil {
		logger.Warning(err)
	}

	return nil
}

type Config struct {
	Version  string
	Config   json.RawMessage
	UpdateAt string
}

func loadConfig(filename string) (*Config, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var cfg Config
	err = json.Unmarshal(content, &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func saveConfig(cfg *Config, filename string) error {
	content, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(filename), 0755)
	if err != nil {
		return err
	}

	tmpFile := filename + ".tmp"
	err = ioutil.WriteFile(tmpFile, content, 0644)
	if err != nil {
		return err
	}

	err = os.Rename(tmpFile, filename)
	if err != nil {
		return err
	}

	return nil
}
