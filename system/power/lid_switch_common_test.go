package power

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NBITS(t *testing.T) {
	var data = strconv.IntSize
	assert.Equal(t, NBITS(data), 1)
	data = strconv.IntSize + 1
	assert.Equal(t, NBITS(data), 2)
}

func Test_LONG(t *testing.T) {
	var data = 1
	assert.Equal(t, LONG(data), 0)
	data = strconv.IntSize + 1
	assert.Equal(t, LONG(data), 1)
	data = strconv.IntSize * 2
	assert.Equal(t, LONG(data), 2)
}

func Test_OFF(t *testing.T) {
	var data = 1
	assert.Equal(t, OFF(data), 1)
	data = 33
	assert.Equal(t, OFF(data), data%strconv.IntSize)
}
