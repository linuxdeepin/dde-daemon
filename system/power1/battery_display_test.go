package power1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_rightPercentage(t *testing.T) {
	var data float64
	data = -50.75
	assert.Equal(t, rightPercentage(data), 0.0)
	data = 66.66
	assert.Equal(t, rightPercentage(data), 66.66)
	data = 123.111
	assert.Equal(t, rightPercentage(data), 100.0)
}

func Test_changeBatteryLowByBatteryPercentage(t *testing.T) {
	m := Manager{}

	percentage := lowBatteryThreshold - 1
	m.changeBatteryLowByBatteryPercentage(percentage)

	percentage = lowBatteryThreshold + 1
	m.changeBatteryLowByBatteryPercentage(percentage)

	percentage = lowBatteryThreshold + 1
	m.batteryLow = false
	m.changeBatteryLowByBatteryPercentage(percentage)

	percentage = lowBatteryThreshold + 1
	m.batteryLow = true
	m.changeBatteryLowByBatteryPercentage(percentage)
}

func Test_ManagerSimpleFunc(t *testing.T) {
	m := Manager{}

	m.resetBatteryDisplay()
}
