package calltrace

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_sumMemByFile(t *testing.T) {
	const testFile = "testdata/status"
	tests := []struct {
		arg  string
		want int64
		err  bool
	}{
		{"testdata/status", 29180, false},
		{"testdata/xxx", 0, true},
	}

	for _, e := range tests {
		get, err := sumMemByFile(e.arg)
		if e.err {
			assert.NotNil(t, err)
			continue
		}
		assert.Equal(t, e.want, get)
	}
}

func Test_getInterge(t *testing.T) {
	tests := []struct {
		arg  string
		want int64
		err  bool
	}{
		{"VmPTE:	     664 kB", 664, false},
		{"test xxx", 0, true},
		{"mmm 2020 M", 2020, false},
	}

	for _, e := range tests {
		get, err := getInterge(e.arg)
		if e.err {
			assert.NotNil(t, err)
			continue
		}
		assert.Equal(t, e.want, get)
	}
}
