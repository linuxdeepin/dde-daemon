package power

import (
	"io/ioutil"
	"os"
	"strings"
)

const (
	powerPolicyFile                   = "/sys/module/pcie_aspm/parameters/policy"
	powerDpmForcePerformanceLevelFile = "/sys/class/drm/card0/device/power_dpm_force_performance_level"
)

func writeToFile(file, data string) error {
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	_, err = f.Write([]byte(data))
	if err1 := f.Close(); err1 != nil && err == nil {
		err = err1
	}
	return err
}

func getPowerPolicy() string {
	contents, err := ioutil.ReadFile(powerPolicyFile)
	if err != nil {
		return ""
	}

	policySlice := strings.Split(string(contents), " ")
	for _, v := range policySlice {
		if i := strings.IndexByte(v, '['); i < 0 {
			continue
		} else {
			if j := strings.LastIndexByte(v, ']'); j > 0 {
				return v[i+1 : j]
			}
		}
	}
	return ""
}

func setPowerPolicy(policy string) error {
	return writeToFile(powerPolicyFile, policy)
}

func getPowerDpmForcePerformanceLevel() string {
	contents, err := ioutil.ReadFile(powerDpmForcePerformanceLevelFile)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(contents))
}

func setPowerDpmForcePerformanceLevel(level string) error {
	return writeToFile(powerDpmForcePerformanceLevelFile, level)
}
