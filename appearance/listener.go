// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package appearance

// #cgo pkg-config:  gtk+-3.0
// #cgo CFLAGS: -W -Wall -fstack-protector-all -fPIC
// #include <stdlib.h>
// #include "cursor.h"
import "C"

func (*Manager) listenCursorChanged() {
	C.handle_gtk_cursor_changed()
}

func (*Manager) endCursorChangedHandler() {
	C.end_cursor_changed_handler()
}
