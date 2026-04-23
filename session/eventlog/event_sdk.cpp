// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

#include <dlfcn.h>
#include <fstream>
#include <iostream>
#include <string>

#include "event_sdk.h"

using namespace std;

// Enable logging for current system version (debug only, remove before release)
// #define DDE_EVENTLOG_DEBUG_ENABLE_CURRENT_VERSION

typedef bool (*Initialize)(const std::string &packagename, bool enable_sig);
typedef void (*WriteEventLog)(const std::string &eventdata);

WriteEventLog fp_writeEventLog = nullptr;
void *handler = nullptr;

// Check if event logging should be enabled based on system edition
// Only UosProfessional is enabled by default
// Reads /etc/os-version to determine edition
static bool shouldEnableEventLog()
{
#ifdef DDE_EVENTLOG_DEBUG_ENABLE_CURRENT_VERSION
    // Debug mode: enable for current system version
    return true;
#else
    // Production mode: only enable for UosProfessional edition
    // Read /etc/os-version and check EditionName
    ifstream osVersion("/etc/os-version");
    if (!osVersion.is_open()) {
        return false;
    }

    string line;
    while (getline(osVersion, line)) {
        // Look for EditionName=Professional
        if (line.find("EditionName=") == 0) {
            string edition = line.substr(12); // Skip "EditionName="
            osVersion.close();
            return edition == "Professional";
        }
    }
    osVersion.close();
    return false;
#endif
}

int InitEventSDK() {
    if (!shouldEnableEventLog()) {
        return EVENT_LOG_DISABLED;
    }

    handler = dlopen("/usr/lib/libdeepin-event-log.so", RTLD_LAZY);
    if (handler == NULL) {
        std::cout << "dlerror = " << dlerror() << std::endl;
        return EVENT_LOG_ERROR;
    }
    auto fp_initialize = (Initialize)dlsym(handler, "Initialize");
    fp_writeEventLog = (WriteEventLog)dlsym(handler, "WriteEventLog");
    bool ret = fp_initialize("org.deepin.dde.daemon", false);
    if (!ret) {
        dlclose(handler);
        fp_initialize = nullptr;
        fp_writeEventLog = nullptr;
        return EVENT_LOG_ERROR;
    }
    return EVENT_LOG_SUCCESS;
}

void CloseEventLog() {
    if (handler) {
        dlclose(handler);
        handler = nullptr;
    }
}

void writeEventLog(const char *log, int isDebug) {
    if (!shouldEnableEventLog()) {
        return;
    }

    if (isDebug) {
        cout << log << endl;
    }
    if (fp_writeEventLog) fp_writeEventLog(log);
}
