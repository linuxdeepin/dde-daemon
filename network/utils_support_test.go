package network

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	dbus "github.com/godbus/dbus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_isStringInArray(t *testing.T) {
	entries := []struct {
		arg1     string
		arg2     []string
		expected bool
	}{
		{"a", []string{"a", "b"}, true},
		{"b", []string{"a", "c"}, false},
	}

	for _, e := range entries {
		b := isStringInArray(e.arg1, e.arg2)
		assert.Equal(t, e.expected, b)
	}
}

func Test_isDBusPathInArray(t *testing.T) {
	entries := []struct {
		arg1     dbus.ObjectPath
		arg2     []dbus.ObjectPath
		expected bool
	}{
		{"/a/b", []dbus.ObjectPath{"/a/b", "/b/c"}, true},
		{"/b/c", []dbus.ObjectPath{"/a/c", "/c/d"}, false},
	}
	for _, e := range entries {
		b := isDBusPathInArray(e.arg1, e.arg2)
		assert.Equal(t, e.expected, b)
	}
}

func Test_isInterfaceNil(t *testing.T) {
	var nilSlice []byte
	entries := []struct {
		arg      interface{}
		expected bool
	}{
		{nil, true},
		{nilSlice, true},
		{[]byte{}, false},
		{1, false},
	}
	for _, e := range entries {
		b := isInterfaceNil(e.arg)
		assert.Equal(t, e.expected, b)
	}
}

func Test_marshalJSON(t *testing.T) {
	b := []string{"go", "c", "java"}
	_, err := marshalJSON(b)
	assert.Nil(t, err)
}

func Test_execWithIO(t *testing.T) {
	want := "hi..."
	_, _, out, _, err := execWithIO("echo", want)
	if err != nil {
		return
	}

	ret := make([]byte, 0, 5)
	b := bytes.NewBuffer(ret)
	_, err = io.Copy(b, out)
	require.NoError(t, err)
	get := strings.TrimSpace(b.String())
	assert.Equal(t, want, get)
}

func Test_isWirelessDeviceSupportHotspot(t *testing.T) {
	want := false
	get := isWirelessDeviceSupportHotspot("")
	assert.Equal(t, want, get)
}

func Test_getRedirectFromResponse(t *testing.T) {
	detectUrl := "127.0.0.1:9531"
	ln, err := net.Listen("tcp", detectUrl)
	require.NoError(t, err)
	defer ln.Close()

	http.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test"))
	})
	go http.Serve(ln, nil)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	res, err := client.Get(detectUrl)
	if err != nil {
		return
	}

	portal, err := getRedirectFromResponse(res, detectUrl)
	if err != nil {
		return
	}

	assert.Equal(t, "", portal)
}
