package power

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
