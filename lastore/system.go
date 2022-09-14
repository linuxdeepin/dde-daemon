// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package lastore

import (
	"encoding/json"
	"os"
)

const (
	DownloadJobType           = "download"
	InstallJobType            = "install"
	RemoveJobType             = "remove"
	UpdateJobType             = "update"
	DistUpgradeJobType        = "dist_upgrade"
	PrepareDistUpgradeJobType = "prepare_dist_upgrade"
	UpdateSourceJobType       = "update_source"
	CustomUpdateJobType       = "custom_update"
	CleanJobType              = "clean"
	SystemUpgradeJobType      = "system_upgrade"
)

type Status string

const (
	ReadyStatus   Status = "ready"
	RunningStatus Status = "running"
	FailedStatus  Status = "failed"
	SucceedStatus Status = "succeed"
	PausedStatus  Status = "paused"

	EndStatus = "end"
)
const varLibDir = "/var/lib/lastore"

func decodeJson(fpath string, d interface{}) error {
	f, err := os.Open(fpath)
	if err != nil {
		return err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			logger.Warning(err)
		}
	}()

	return json.NewDecoder(f).Decode(&d)
}
