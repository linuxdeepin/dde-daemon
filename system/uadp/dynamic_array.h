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

#include <assert.h>
#include <stdio.h>
#include <stdlib.h>

#define INITIAL_CAP 10
#define INCREMENT_SIZE 10

struct DynamicArray {
    int *pBase;    //存储的是数组第一个元素的地址
    int cap;       //数组所能容纳的最大元素的个数
    int cnt;       //当前数组有效元素的个数
};

int init_array(struct DynamicArray *pArr);
void destroy_array(struct DynamicArray *pArr);
void append(struct DynamicArray *pArr, int val);
int is_exist(struct DynamicArray *pArr, int val);