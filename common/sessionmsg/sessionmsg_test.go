// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package sessionmsg

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBodyNotify_MessageType(t *testing.T) {
	var b BodyNotify
	assert.Equal(t, MessageTypeNotify, b.MessageType())
}

func TestNewMessage_Notify(t *testing.T) {
	body := &BodyNotify{Icon: "dialog-info", AppName: "test-app"}
	m := NewMessage(true, body)

	assert.Equal(t, MessageTypeNotify, m.Type)
	assert.True(t, m.OnlyActive)
	assert.True(t, m.Body == body)
}

func TestNewMessage_OnlyActiveFalse(t *testing.T) {
	body := &BodyNotify{Icon: "x"}
	m := NewMessage(false, body)

	assert.False(t, m.OnlyActive)
	assert.Equal(t, MessageTypeNotify, m.Type)
	assert.True(t, m.Body == body)
}

func TestMessage_UnmarshalJSON_Notify(t *testing.T) {
	raw := `{"OnlyActive":true,"Type":1,"Body":{"Icon":"dialog-info","AppName":"test-app","ExpireTimeout":5000}}`
	var m Message
	err := m.UnmarshalJSON([]byte(raw))
	require.NoError(t, err)

	assert.Equal(t, MessageTypeNotify, m.Type)
	assert.True(t, m.OnlyActive)

	bn, ok := m.Body.(*BodyNotify)
	assert.True(t, ok, "Body should be *BodyNotify")
	assert.Equal(t, "dialog-info", bn.Icon)
	assert.Equal(t, "test-app", bn.AppName)
	assert.Equal(t, 5000, bn.ExpireTimeout)
}

func TestMessage_UnmarshalJSON_UnknownType(t *testing.T) {
	// Type=99 不是已知类型，switch 不匹配，Body 保持 nil，无错误
	raw := `{"OnlyActive":false,"Type":99,"Body":{}}`
	var m Message
	err := m.UnmarshalJSON([]byte(raw))
	assert.NoError(t, err)
	assert.Equal(t, MessageType(99), m.Type)
	assert.False(t, m.OnlyActive)
	assert.Nil(t, m.Body)
}

func TestMessage_UnmarshalJSON_InvalidJSON(t *testing.T) {
	raw := `{invalid json`
	var m Message
	err := m.UnmarshalJSON([]byte(raw))
	assert.Error(t, err)
}

func TestMessage_UnmarshalJSON_BodyUnmarshalError(t *testing.T) {
	// Type=1 触发 BodyNotify 解析，但 ExpireTimeout 传字符串无法转为 int
	raw := `{"OnlyActive":true,"Type":1,"Body":{"ExpireTimeout":"not-a-number"}}`
	var m Message
	err := m.UnmarshalJSON([]byte(raw))
	assert.Error(t, err)
	assert.Nil(t, m.Body)
}

func TestMessage_MarshalUnmarshal_RoundTrip(t *testing.T) {
	orig := NewMessage(true, &BodyNotify{
		Icon:          "dialog-warning",
		AppName:       "roundtrip-app",
		ExpireTimeout: -1,
	})
	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var decoded Message
	err = decoded.UnmarshalJSON(data)
	require.NoError(t, err)
	assert.Equal(t, orig.Type, decoded.Type)
	assert.Equal(t, orig.OnlyActive, decoded.OnlyActive)

	bn, ok := decoded.Body.(*BodyNotify)
	assert.True(t, ok)
	assert.Equal(t, "dialog-warning", bn.Icon)
	assert.Equal(t, "roundtrip-app", bn.AppName)
	assert.Equal(t, -1, bn.ExpireTimeout)
}

func TestLocalizeStr_String_Nil(t *testing.T) {
	var ls *LocalizeStr
	assert.Equal(t, "", ls.String())
}

func TestLocalizeStr_String_NoArgs(t *testing.T) {
	ls := &LocalizeStr{Format: "hello world"}
	assert.Equal(t, "hello world", ls.String())
}

func TestLocalizeStr_String_WithArgs(t *testing.T) {
	ls := &LocalizeStr{Format: "Hello %s, welcome to %s", Args: []string{"Alice", "DDE"}}
	assert.Equal(t, "Hello Alice, welcome to DDE", ls.String())
}

func TestLocalizeStr_String_SingleArg(t *testing.T) {
	ls := &LocalizeStr{Format: "value is %s", Args: []string{"42"}}
	assert.Equal(t, "value is 42", ls.String())
}

func TestMessageType_Constants(t *testing.T) {
	assert.Equal(t, MessageType(1), MessageTypeNotify)
}
