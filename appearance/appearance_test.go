package appearance

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_hexColorToXsColor(t *testing.T) {
	var tests = []struct {
		hex string
		xs  string
		err bool
	}{
		{"#ffffff", "65535,65535,65535,65535", false},
		{"#ffffffff", "65535,65535,65535,65535", false},
		{"#ff69b4", "65535,26985,46260,65535", false},
		{"#00000000", "0,0,0,0", false},
		{"abc", "", true},
		{"#FfFff", "", true},
		{"", "", true},
	}

	for _, test := range tests {
		xsColor, err := hexColorToXsColor(test.hex)
		assert.Equal(t, err != nil, test.err)
		assert.Equal(t, xsColor, test.xs)
	}
}

func Test_xsColorToHexColor(t *testing.T) {
	var tests = []struct {
		hex string
		xs  string
		err bool
	}{
		{"#FFFFFF", "65535,65535,65535,65535", false},
		{"#FF69B4", "65535,26985,46260,65535", false},
		{"", "", true},
		{"", "123,456,678", true},
		{"", "-1,-2,03,45", true},
		{"", "65535,26985,46260,65535,", true},
		{"", "65535,26985,46260,165535", true},
	}

	for _, test := range tests {
		hexColor, err := xsColorToHexColor(test.xs)
		assert.Equal(t, err != nil, test.err)
		assert.Equal(t, hexColor, test.hex)
	}
}

func Test_byteArrayToHexColor(t *testing.T) {
	var tests = []struct {
		byteArray [4]byte
		hex       string
	}{
		{[4]byte{0xff, 0xff, 0xff}, "#FFFFFF00"},
		{[4]byte{0xff, 0xff, 0xff, 0xff}, "#FFFFFF"},
		{[4]byte{0xff, 0x69, 0xb4}, "#FF69B400"},
		{[4]byte{0xff, 0x69, 0xb4, 0xff}, "#FF69B4"},
	}

	for _, test := range tests {
		hexColor := byteArrayToHexColor(test.byteArray)
		assert.Equal(t, hexColor, test.hex)
	}
}

func Test_parseHexColor(t *testing.T) {
	var tests = []struct {
		byteArray [4]byte
		hex       string
	}{
		{[4]byte{0xff, 0xff, 0xff}, "#FFFFFF00"},
		{[4]byte{0xff, 0xff, 0xff, 0xff}, "#FFFFFF"},
		{[4]byte{0xff, 0x69, 0xb4}, "#FF69B400"},
		{[4]byte{0xff, 0x69, 0xb4, 0xff}, "#FF69B4"},
	}

	for _, test := range tests {
		byteArray, err := parseHexColor(test.hex)
		assert.Nil(t, err)
		assert.Equal(t, byteArray, test.byteArray)
	}
}
