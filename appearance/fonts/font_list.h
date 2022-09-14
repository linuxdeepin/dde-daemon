// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

#ifndef __FONT_LIST_H__
#define __FONT_LIST_H__

typedef struct _FcInfo {
	char *family;
	char *familylang;
	/* char *fullname; */
	/* char *fullnamelang; */
	char *style;
	char *lang;
	char *spacing;
	/* char *filename; */
} FcInfo;

int fc_cache_update ();
FcInfo *list_font_info (int *num);
void free_font_info_list(FcInfo *list, int num);

char* font_match(char* family);

#endif
