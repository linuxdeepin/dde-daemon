package evdev

/*
#include <linux/version.h>
#include <linux/input.h>
#include <fcntl.h>
#include <unistd.h>
#include <stdio.h>
#include <stdlib.h>
#include <errno.h>

int numLockState;
int capsState;

int getNumLockState() {
	return numLockState;
}

int getCapsState() {
	return capsState;
}

int print_events(int fd)
{
    struct input_event ev[64];
    int i, rd;
    fd_set rdfs;

    FD_ZERO(&rdfs);
    FD_SET(fd, &rdfs);

    rd = read(fd, ev, sizeof(ev));
    if (rd < (int) sizeof(struct input_event)) {
        close(fd);
        return 1;
    }

    for (i = 0; i < rd / sizeof(struct input_event); i++) {
        //+ NumLock
        if (0 == ev[i].code && 17 == ev[i].type) {
			numLockState = ev[i].value;
        }

        //+ Caps
        if (1 == ev[i].code && 17 == ev[i].type) {
			capsState = ev[i].value;
        }
    }

    ioctl(fd, EVIOCGRAB, (void*)0);
    return EXIT_SUCCESS;
}

void run()
{
    int fd;
    char *filename = "/dev/input/event2";

    if ((fd = open(filename, O_RDONLY)) < 0) {
        if (errno == EACCES) {
            return;
        }
    }

    while (1) {
        if (1 == print_events(fd))
            return;
    }
}
*/
import "C"

import (
	"sync"

	dbus "pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/dbusutil"
)

const (
	dbusServiceName = "com.deepin.system.Evdev"
	dbusPath        = "/com/deepin/system/Evdev"
	dbusInterface   = dbusServiceName
)

type Manager struct {
	service *dbusutil.Service
	PropsMu sync.RWMutex

	methods *struct {
		GetNumLockState  func() `out:"state"`
		GetCapsLockState func() `out:"state"`
	}
}

func (m *Manager) GetInterfaceName() string {
	return dbusInterface
}

func newManager(service *dbusutil.Service) *Manager {
	var m = &Manager{
		service: service,
	}
	return m
}

func (m *Manager) GetNumLockState() (int32, *dbus.Error) {
	state := C.getNumLockState()
	return int32(state), nil
}

func (m *Manager) GetCapsLockState() (int32, *dbus.Error) {
	state := C.getCapsState()
	return int32(state), nil
}

func (m *Manager) runEvtest() {
	C.run()
}
