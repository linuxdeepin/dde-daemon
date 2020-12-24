package power

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NBITS(t *testing.T) {
	var data = 64
	assert.Equal(t, NBITS(data), 1)
	data = 65
	assert.Equal(t, NBITS(data), 2)
}

func Test_LONG(t *testing.T) {
	var data = 1
	assert.Equal(t, LONG(data), 0)
	data = 65
	assert.Equal(t, LONG(data), 1)
	data = 128
	assert.Equal(t, LONG(data), 2)
}

func Test_OFF(t *testing.T) {
	var data = 1
	assert.Equal(t, OFF(data), 1)
	data = 5
	assert.Equal(t, OFF(data), 5)
}

func Test_testBit(t *testing.T) {
	data := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	assert.Equal(t, testBit(5, data), false)
	assert.Equal(t, testBit(64, data), true)
}
