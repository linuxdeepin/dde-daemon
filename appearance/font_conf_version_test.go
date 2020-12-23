package appearance

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_isVersionRight(t *testing.T) {
	assert.True(t, isVersionRight("1.4", "testdata/fontVersionConf"))
}
