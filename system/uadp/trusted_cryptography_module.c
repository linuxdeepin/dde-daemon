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

#include "trusted_cryptography_module.h"
#include "dynamic_array.h"

#define RANDOM_INTERVAL 50
#define KEY_BASE 0x81010003
static struct DynamicArray dynamic_array;

// 初始化可信计算环境，根据传入类型，与可信设备建立连接，返回连接信息句柄。
// start_tc函数是一些调用过程的按序封装
TC_RC
start_tc(TC_HANDLE *tc_handle, uint32_t *persist_index) {
    TC_RC rc;
    uint32_t  primary_index;
    uint32_t  key_index;
    *persist_index = generate_persist_key();

    rc = TC_Start("tabrmd",
                  tc_handle);
    if (rc != TC_SUCCESS) {
        printf("Failed to start TC device\n");
        error(tc_handle);
        return rc;
    }

    rc = TC_CreatePrimary(*tc_handle,
                          TC_SHA256,
                          TC_TPM2_RH_ENDORSEMENT,
                          NULL,
                          TC_RSA,
                          NULL,
                          &primary_index);
    if (rc != TC_SUCCESS) {
        printf("Failed to createPrimary\n");
        error(tc_handle);
        return rc;
    }

    rc = TC_Create(*tc_handle,
                   TC_SHA256,
                   primary_index,
                   NULL,
                   TC_SYMMETRIC,
                   NULL,
                   &key_index);
    if (rc != TC_SUCCESS) {
        printf("Failed to create key\n");
        error(tc_handle);
        return rc;
    }

    rc = TC_Load(*tc_handle,
                 key_index,
                 NULL);
    if (rc != TC_SUCCESS) {
        printf("Failed to load key\n");
        error(tc_handle);
        return rc;
    }

    rc = TC_EvictControl(*tc_handle,
                         true,
                         *persist_index,
                         key_index,
                         TC_TPM2_RH_OWNER,
                         NULL);
    if (rc != TC_SUCCESS) {
        printf("Failed to enable evict control key\n");
        error(tc_handle);
        return rc;
    }
    return rc;
}

TC_RC
stop_tc(TC_HANDLE *tc_handle) {
    TC_RC rc;

    rc = TC_End(tc_handle);
    if (rc != TC_SUCCESS) {
        printf("Failed to stop TC device\n");
    }
    return rc;
}

// 随机生成持久密钥
uint32_t
generate_persist_key() {
    uint32_t persist_key = 0;
    srand((unsigned)time(NULL));
    do {
        int off_set = rand() % RANDOM_INTERVAL;
        // 检查该随机密钥是否已经存在。若存在，重新计算
        if (!is_exist(&dynamic_array, off_set)) {
            append(&dynamic_array, off_set);
            persist_key = KEY_BASE + off_set;
            break;
        }
    } while(1);
    return persist_key;
}

// 加密数据
TC_RC
encrypt(TC_HANDLE *tc_handle, const uint32_t *persist_key, const char *plain, int len, TC_BUFFER *ciphter) {
    // 首先初始化ciphter
    init_tc_buffer(ciphter);

    TC_BUFFER bfDecryptText;
    bfDecryptText.buffer = (unsigned char *)malloc(len);
    memcpy(bfDecryptText.buffer, plain, len);
    bfDecryptText.size = len;

    TC_RC rc;
    rc = TC_Encrypt(*tc_handle,
                    *persist_key, // 若以持久索引persist_key来加密数据，存在问题。
                    NULL,
                    TC_SYMMETRIC,
                    &bfDecryptText,
                    ciphter);
    if (rc != TC_SUCCESS) {
        error(tc_handle);
    }
    return rc;
}

// 解密数据
TC_RC
decrypt(TC_HANDLE *tc_handle, const uint32_t *persist_key, const TC_BUFFER *ciphter, TC_BUFFER *plain) {
    // 首先初始化plain
    init_tc_buffer(plain);

    TC_RC rc;
    rc = TC_Decrypt(*tc_handle,
                    *persist_key, // 若以持久索引persist_key来解密数据，存在问题。
                    NULL,
                    TC_SYMMETRIC,
                    ciphter,
                    plain);
    if (rc != TC_SUCCESS) {
        error(tc_handle);
    }
    return rc;
}

void
error(TC_HANDLE *tc_handle) {
    TC_RC rc;
    rc = TC_End(tc_handle);
    if (rc != TC_SUCCESS) {
        printf("Failed to stop TC device\n");
    }
}

void
init_tc_buffer(TC_BUFFER *pBuf) {
    if (NULL == pBuf) {
        return;
    } else {
        if (NULL != pBuf->buffer) {
            free(pBuf->buffer);
            pBuf->buffer = NULL;
        }
        pBuf->size = 0;
        pBuf = NULL;
    }
}