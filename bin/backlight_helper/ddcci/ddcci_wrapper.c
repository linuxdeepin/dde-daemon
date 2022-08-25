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
#include <stdio.h>
#include "ddcci_wrapper.h"
typedef int (*ddca_free_all_displays)();

ddca_free_all_displays fp_ddca_free_all_displays = NULL;

int InitDDCCISo(const char *path) {
    void *handler = dlopen(path, RTLD_LAZY);
    if (handler == NULL) {
        return -1;
    }
    fp_ddca_free_all_displays = (ddca_free_all_displays)dlsym(handler, "ddca_free_all_displays");
    if (fp_ddca_free_all_displays == NULL) {
        if (handler) {
            dlclose(handler);
        }
        return -2;
    }
    return 0;
}

int freeAllDisplaysWrapper() {
    if (fp_ddca_free_all_displays) {
        return fp_ddca_free_all_displays();
    }
}
