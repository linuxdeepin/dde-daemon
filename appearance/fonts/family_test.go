package fonts

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_IsFontFamily(t *testing.T) {
	tests := []struct {
		arg      string
		excepted bool
	}{
		{"monospace", true},
		{"sans-serif", true},
		{"courier", false},
		{"hack", false},
	}

	for _, e := range tests {
		v := IsFontFamily(e.arg)
		assert.Equal(t, e.excepted, v)
	}
}

func Test_IsFontSizeValid(t *testing.T) {
	tests := []struct {
		arg      float64
		excepted bool
	}{
		{10, true},
		{32, false},
		{5, false},
	}

	for _, e := range tests {
		v := IsFontSizeValid(e.arg)
		assert.Equal(t, e.excepted, v)
	}
}

func Test_SetFamily(t *testing.T) {
	tests := []struct {
		arg1 string
		arg2 string
		arg3 float64
		err  bool
	}{
		{"sans-serif", "monospace", 10, false},
		{"hack", "monospace", 32, true},
	}

	DeepinFontConfig = "testdata/test.conf"
	for _, e := range tests {
		v := SetFamily(e.arg1, e.arg2, e.arg3)
		if e.err {
			assert.NotNil(t, v)
			return
		}

		_, err := ioutil.ReadFile(DeepinFontConfig)
		assert.Nil(t, err)
	}
}
