package displaycfg

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/godbus/dbus"
	"pkg.deepin.io/lib/dbusutil"
)

const (
	dbusServiceName   = "com.deepin.system.DisplayCfg"
	dbusInterfaceName = dbusServiceName
	dbusPath          = "/com/deepin/system/DisplayCfg"
	configFilePath    = "/var/lib/dde-daemon/display/config.json"
)

//go:generate dbusutil-gen em -type DisplayCfg

type DisplayCfg struct {
	service *dbusutil.Service
	cfg     *Config
	signals *struct {
		Updated struct {
			updateAt string
		}
	}
}

func newDisplayCfg(service *dbusutil.Service) *DisplayCfg {
	d := &DisplayCfg{
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

func (d *DisplayCfg) GetInterfaceName() string {
	return dbusInterfaceName
}

func (d *DisplayCfg) Get() (cfgStr string, busErr *dbus.Error) {
	data, err := json.Marshal(d.cfg)
	if err != nil {
		return "", dbusutil.ToError(err)
	}

	return string(data), nil
}

func (d *DisplayCfg) Set(cfgStr string) *dbus.Error {
	err := d.set(cfgStr)
	return dbusutil.ToError(err)
}

func (d *DisplayCfg) set(cfgStr string) error {
	var cfg Config
	err := json.Unmarshal([]byte(cfgStr), &cfg)
	if err != nil {
		return err
	}

	d.cfg = &cfg

	err = saveConfig(&cfg, configFilePath)
	if err != nil {
		return err
	}

	err = d.service.Emit(d, "Updated", cfg.UpdateAt)
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
