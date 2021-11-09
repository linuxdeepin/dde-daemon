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

#include <stdio.h>
#include <stddef.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

#include <tc/tc_api.h>
#include <tc/tc_type.h>
#include <tc/tc_errcode.h>

TC_RC start_tc(TC_HANDLE *tc_handle, uint32_t *persist_index);
TC_RC stop_tc(TC_HANDLE *tc_handle);
uint32_t generate_persist_key();
TC_RC encrypt(TC_HANDLE *tc_handle, const uint32_t *persist_key, const char *plain, int len, TC_BUFFER *ciphter);
TC_RC decrypt(TC_HANDLE *tc_handle, const uint32_t *persist_key, const TC_BUFFER *ciphter, TC_BUFFER *plain);
void error(TC_HANDLE *tc_handle);
void init_tc_buffer(TC_BUFFER *pBuf);