package appearance

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getThemeAutoName(t *testing.T) {
	assert.Equal(t, getThemeAutoName(true), "deepin")
	assert.Equal(t, getThemeAutoName(false), "deepin-dark")
}
