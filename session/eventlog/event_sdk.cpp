/*
 * Copyright (C) 2019 ~ 2022 Uniontech Software Technology Co.,Ltd
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
#include <dlfcn.h>
#include <iostream>

#include "event_sdk.h"

using namespace std;

typedef bool (*Initialize)(const std::string &packagename, bool enable_sig);
typedef void (*WriteEventLog)(const std::string &eventdata);

WriteEventLog fp_writeEventLog = nullptr;
void *handler = nullptr;

int InitEventSDK() {
    handler = dlopen("/usr/lib/libdeepin-event-log.so", RTLD_LAZY);
    if (handler == NULL) {
        std::cout << "dlerror = " << dlerror() << std::endl;
        return -1;
    }
    auto fp_initialize = (Initialize)dlsym(handler, "Initialize");
    fp_writeEventLog = (WriteEventLog)dlsym(handler, "WriteEventLog");
    bool ret = fp_initialize("org.deepin.dde.daemon", false);
    if (!ret) {
        dlclose(handler);
        fp_initialize = nullptr;
        fp_writeEventLog = nullptr;
        return -1;
    }
    return 0;
}

void CloseEventLog() {
    dlclose(handler);
}

void writeEventLog(const char *log) {
    cout << log << endl;
    if (fp_writeEventLog) fp_writeEventLog(log);
}