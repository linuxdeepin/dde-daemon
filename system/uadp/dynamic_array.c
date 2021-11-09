/*
 * Copyright (C) 2013 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     ganjing <ganjing@uniontech.com>
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

#include "dynamic_array.h"
#include <assert.h>

int
init_array(struct DynamicArray *pArr) {
    int ret = -1;
    pArr->pBase = (int *)malloc(sizeof(int) * INITIAL_CAP);
    if (NULL != pArr->pBase) {
        pArr->cap = INITIAL_CAP;
        pArr->cnt = 0;
        ret = 0;
    }
    return ret;
}

void
destroy_array(struct DynamicArray *pArr) {
    if (NULL == pArr->pBase) {
        return;
    }
    free(pArr->pBase);
    pArr->cap = 0;
    pArr->cnt = 0;
    free(pArr);
    return;
}

void
append(struct DynamicArray *pArr, int val) {
    if (pArr->cnt == pArr->cap) {
        pArr->pBase = realloc(pArr->pBase, INCREMENT_SIZE * sizeof(int));
        pArr->cap += INCREMENT_SIZE;
    }
    pArr->pBase[pArr->cnt] = val;
    pArr->cnt++;
}

int
is_exist(struct DynamicArray *pArr, int val) {
    assert(pArr->pBase != NULL);

    int ret = -1;
    // 数组中没有元素时，val肯定是不存在pArr中的
    if (pArr->cnt == 0) {
        return 0;
    }

    for (int i = 0; i < pArr->cnt; i++) {
        if (pArr->pBase[i] == val) {
            ret = 0;
            break;
        }
    }
    return ret;
}