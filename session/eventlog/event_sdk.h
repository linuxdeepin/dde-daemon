// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

#ifndef EVENT_SDK_H
#define EVENT_SDK_H

#ifdef __cplusplus
extern "C" {
#endif

// Return codes for InitEventSDK
#define EVENT_LOG_SUCCESS  0  // Initialization succeeded
#define EVENT_LOG_DISABLED 1  // Logging disabled (non-Professional edition)
#define EVENT_LOG_ERROR   -1  // Initialization failed

int InitEventSDK();
void writeEventLog(const char *log, int isDebug);
void CloseEventLog();


#ifdef __cplusplus
}
#endif
#endif // EVENT_SDK_H