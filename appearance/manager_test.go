package appearance

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_genMonitorKeyString(t *testing.T) {
	assert.Equal(t, genMonitorKeyString("abc", 123), "abc&&123")
	assert.Equal(t, genMonitorKeyString("abc", "def"), "abc&&def")
}
