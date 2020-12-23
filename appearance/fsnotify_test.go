package appearance

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_hasEventOccurred(t *testing.T) {
	var shellStr = []string{"/bin/sh", "/bin/bash",
		"/bin/zsh", "/usr/bin/zsh",
		"/usr/bin/fish",
	}

	assert.Equal(t, hasEventOccurred("/usr/bin/sh", shellStr), true)
	assert.Equal(t, hasEventOccurred("/usr/lib/deepin", shellStr), false)
}
