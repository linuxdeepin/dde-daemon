/*
 * Copyright (C) 2014 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

//#cgo pkg-config: x11 gtk+-3.0
//#include <X11/Xlib.h>
//#include <gtk/gtk.h>
//void init(){XInitThreads();gtk_init(0,0);}
import "C"
import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	login1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.login1"

	"github.com/godbus/dbus"
	soundthemeplayer "github.com/linuxdeepin/go-dbus-factory/com.deepin.api.soundthemeplayer"
	"pkg.deepin.io/dde/api/soundutils"
	"pkg.deepin.io/dde/api/userenv"
	"pkg.deepin.io/dde/daemon/loader"
	"pkg.deepin.io/lib/dbusutil"
	. "pkg.deepin.io/lib/gettext"
	"pkg.deepin.io/lib/log"
	"pkg.deepin.io/lib/proxy"
	"pkg.deepin.io/lib/utils"
	"pkg.deepin.io/lib/xdg/basedir"
)

var logger = log.NewLogger("daemon/dde-session-daemon")
var hasDDECookie bool

func isInShutdown() bool {
	bus, err := dbus.SystemBus()
	if err != nil {
		return false
	}

	manager := login1.NewManager(bus)

	val, err := manager.PreparingForShutdown().Get(0)
	if err != nil {
		return false
	}

	return val
}

func allowRun() bool {
	if os.Getenv("DDE_SESSION_PROCESS_COOKIE_ID") != "" {
		hasDDECookie = true
		return true
	}

	systemBus, err := dbus.SessionBus()
	if err != nil {
		logger.Warning(err)
		os.Exit(1)
	}
	sessionManagerObj := systemBus.Object("com.deepin.SessionManager",
		"/com/deepin/SessionManager")
	var allowRun bool
	err = sessionManagerObj.Call("com.deepin.SessionManager.AllowSessionDaemonRun",
		dbus.FlagNoAutoStart).Store(&allowRun)
	if err != nil {
		logger.Warning(err)
		return true
	}

	return allowRun
}

var _options struct {
	verbose  bool
	logLevel string
	list     string
	enable   string
	disable  string
	ignore   bool
	force    bool

	enablingModules []string
	disableModules  []string
}

func toLogLevel(name string) (log.Priority, error) {
	name = strings.ToLower(name)
	logLevel := log.LevelInfo
	var err error
	switch name {
	case "":
		logLevel = log.LevelInfo
	case "error":
		logLevel = log.LevelError
	case "warn":
		logLevel = log.LevelWarning
	case "info":
		logLevel = log.LevelInfo
	case "debug":
		logLevel = log.LevelDebug
	case "no":
		logLevel = log.LevelDisable
	default:
		err = fmt.Errorf("%s is not support", name)
	}

	return logLevel, err
}

func init() {
	rand.Seed(time.Now().UnixNano())

	// -v | -verbose
	const verboseUsage = "Show much more message, shorthand for --loglevel debug."
	flag.BoolVar(&_options.verbose, "v", false, verboseUsage)
	flag.BoolVar(&_options.verbose, "verbose", false, verboseUsage)

	// -l | -loglevel
	const logLevelUsage = "Set log level, possible value is error/warn/info/debug/no, info is default"
	flag.StringVar(&_options.logLevel, "l", "", logLevelUsage)
	flag.StringVar(&_options.logLevel, "loglevel", "", logLevelUsage)

	// -f | -force
	const forceUsage = "Force start disabled module."
	flag.BoolVar(&_options.force, "f", false, forceUsage)
	flag.BoolVar(&_options.force, "force", false, forceUsage)

	// -i | -ignore
	const ignoreUsage = "Ignore missing modules."
	flag.BoolVar(&_options.ignore, "i", true, ignoreUsage)
	flag.BoolVar(&_options.ignore, "ignore", true, ignoreUsage)

	// -list
	flag.StringVar(&_options.list, "list", "",
		"List all the modules or the dependencies of one module. The argument can be all or the name of the module.")

	// -enable
	flag.StringVar(&_options.enable, "enable", "",
		"Enable modules and their dependencies, ignore settings.")

	// -disable
	flag.StringVar(&_options.disable, "disable", "", "Disable modules, ignore settings.")
}

func main() {
	logger.SetLogLevel(log.LevelInfo)
	if !allowRun() {
		logger.Warning("session manager does not allow me to run")
		os.Exit(1)
	}

	if isInShutdown() {
		logger.Warning("system is in shutdown, no need to run")
		os.Exit(1)
	}

	flag.Parse()
	InitI18n()
	Textdomain("dde-daemon")

	if _options.verbose {
		_options.logLevel = "debug"
	}

	logLevel, err := toLogLevel(_options.logLevel)
	if err != nil {
		logger.Warning("failed to parse loglevel:", err)
		os.Exit(1)
	}

	if _options.enable != "" {
		_options.enablingModules = strings.Split(_options.enable, ",")
	}
	if _options.disable != "" {
		_options.disableModules = strings.Split(_options.disable, ",")
	}

	usr, err := user.Current()
	if err == nil {
		_ = os.Chdir(usr.HomeDir)
	}

	C.init()
	proxy.SetupProxy()

	app := NewSessionDaemon(daemonSettings, logger)

	service, err := dbusutil.NewSessionService()
	if err != nil {
		logger.Fatal(err)
	}

	if err = app.register(service); err != nil {
		logger.Info(err)
		os.Exit(0)
	}

	loader.SetService(service)

	if _options.logLevel == "" &&
		(utils.IsEnvExists(log.DebugLevelEnv) || utils.IsEnvExists(log.DebugMatchEnv)) {
		logger.Info("Log level is none and debug env exists, so do not call loader.SetLogLevel")
	} else {
		logger.Info("App log level:", _options.logLevel)
		// set all modules log level to logLevel
		loader.SetLogLevel(logLevel)
	}

	// Ensure each module and mainloop in the same thread
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if _options.list != "" {
		err = app.listModule(_options.list)
		if err != nil {
			logger.Warning(err)
			os.Exit(1)
		}
		os.Exit(0)
	} else if len(_options.enablingModules) > 0 {
		err = app.enableModules(_options.enablingModules)
	} else if len(_options.disableModules) > 0 {
		err = app.disableModules(_options.disableModules)
	} else {
		app.execDefaultAction()
	}

	if err != nil {
		logger.Warning(err)
		os.Exit(1)
	}

	err = migrateUserEnv()
	if err != nil {
		logger.Warning("failed to migrate user env:", err)
	}

	err = syncConfigToSoundThemePlayer()
	if err != nil {
		logger.Warning(err)
	}

	runMainLoop()
}

// migrate user env from ~/.pam_environment to ~/.dde-env
func migrateUserEnv() error {
	_, err := os.Stat(userenv.DefaultFile())
	if os.IsNotExist(err) {
		// when ~/.dde-env does not exist, read ~/.pam_environment,
		// remove the key we set before, and save it back.
		pamEnvFile := filepath.Join(basedir.GetUserHomeDir(), ".pam_environment")
		pamEnv, err := loadPamEnv(pamEnvFile)
		if os.IsNotExist(err) {
			return nil
		} else if err != nil {
			return err
		}

		var reservedPamEnv []pamEnvKeyValue
		for _, kv := range pamEnv {
			switch kv.key {
			case "LANG", "LANGUAGE", "QT_SCALE_FACTOR", "_JAVA_OPTIONS":
			// ignore it
			default:
				reservedPamEnv = append(reservedPamEnv, kv)
			}
		}

		if len(reservedPamEnv) == 0 {
			err = os.Remove(pamEnvFile)
		} else {
			err = savePamEnv(pamEnvFile, reservedPamEnv)
		}
		if err != nil {
			return err
		}

		// save the current env to ~/.dde-env
		currentEnv := make(map[string]string)
		for _, envKey := range []string{"LANG", "LANGUAGE"} {
			envValue, ok := os.LookupEnv(envKey)
			if ok {
				currentEnv[envKey] = envValue
			}
		}
		err = userenv.Save(currentEnv)
		return err
	} else if err != nil {
		return err
	}
	return nil
}

type pamEnvKeyValue struct {
	key, value string
}

func loadPamEnv(filename string) ([]pamEnvKeyValue, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	var result []pamEnvKeyValue
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}

		fields := strings.SplitN(line, "=", 2)
		if len(fields) == 2 {
			result = append(result, pamEnvKeyValue{
				key:   fields[0],
				value: fields[1],
			})
		}
	}
	return result, nil
}

func savePamEnv(filename string, pamEnv []pamEnvKeyValue) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	bw := bufio.NewWriterSize(f, 256)
	for _, kv := range pamEnv {
		_, err = fmt.Fprintf(bw, "%s=%s\n", kv.key, kv.value)
		if err != nil {
			return err
		}
	}

	err = bw.Flush()
	return err
}

func syncConfigToSoundThemePlayer() error {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	loginEnabled := soundutils.CanPlayEvent(soundutils.EventDesktopLogin)
	player := soundthemeplayer.NewSoundThemePlayer(sysBus)
	err = player.EnableSoundDesktopLogin(0, loginEnabled)
	return err
}
