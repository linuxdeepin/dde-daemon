package subthemes

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_IsGtkTheme(t *testing.T) {
	v := ListGtkTheme()

	for _, e := range v {
		b := IsGtkTheme(e.Id)
		assert.Equal(t, true, b)
	}

	b := IsGtkTheme("test.id")
	assert.Equal(t, false, b)
}

func Test_IsIconTheme(t *testing.T) {
	v := ListIconTheme()

	for _, e := range v {
		b := IsIconTheme(e.Id)
		assert.Equal(t, true, b)
	}

	b := IsIconTheme("test.id")
	assert.Equal(t, false, b)
}

func Test_IsCursorTheme(t *testing.T) {
	v := ListCursorTheme()

	for _, e := range v {
		b := IsCursorTheme(e.Id)
		assert.Equal(t, true, b)
	}

	b := IsCursorTheme("test.id")
	assert.Equal(t, false, b)
}

func Test_Themes_GetIds(t *testing.T) {
	v := ListGtkTheme()

	ids := v.GetIds()
	for _, e := range ids {
		assert.NotNil(t, v.Get(e))
	}
}
