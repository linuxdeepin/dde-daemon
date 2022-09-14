// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

#include <gio/gio.h>

#define FILE_MANAGER_MIME_TYPE "inode/directory"

/**
 * Open default file manager via mime type
 * Some file managers custom the directory by user
 **/
int
main(int argc, char *argv[])
{
    GAppInfo *app_info = g_app_info_get_default_for_type(FILE_MANAGER_MIME_TYPE, FALSE);
    if (!app_info) {
        g_error("Failed to get default app for %s", FILE_MANAGER_MIME_TYPE);
        return -1;
    }
    g_debug("Executable: %s\n", g_app_info_get_executable(app_info));
    g_debug("Commandline: %s\n", g_app_info_get_commandline(app_info));

    GError *error = NULL;
    gboolean ret = g_app_info_launch(app_info, NULL, NULL, &error);
    if (error) {
        g_error("Failed to launch %s, error: %s", g_app_info_get_name(app_info), error->message);
        g_error_free(error);
        goto EXIT;
    }

EXIT:
    g_object_unref(app_info);
    return ret?0:-1;
}
