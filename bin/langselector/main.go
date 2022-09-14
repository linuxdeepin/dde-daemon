// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/linuxdeepin/go-lib/gettext"
	"github.com/linuxdeepin/dde-daemon/langselector"
)

func main() {
	gettext.InitI18n()
	gettext.BindTextdomainCodeset("dde-daemon", "UTF-8")
	gettext.Textdomain("dde-daemon")
	langselector.Run()
}
