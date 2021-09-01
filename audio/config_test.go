package audio

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_mapStrStrEqual(t *testing.T) {
	var test = map[string]string{"aaa": "aaa", "bbb": "bbb", "ccc": "ccc"}
	var test1 = map[string]string{"aaa": "aaa", "bbb": "bbb", "ccc": "ccc"}
	var test2 = map[string]string{"aaa": "aaa", "bbb": "bbb", "ccc": "ddd"}

	assert.Equal(t, mapStrStrEqual(test, test1), true)
	assert.Equal(t, mapStrStrEqual(test, test2), false)
}

func Test_NewConfigKeeper(t *testing.T) {
	ck := NewConfigKeeper("testdata/audio-config-keeper.json", "testdata/audio-config-keeper-mute.json")
	assert.NotNil(t, ck)
}

func Test_GetCardAndPortConfig(t *testing.T) {
	ck := NewConfigKeeper("testdata/audio-config-keeper.json", "testdata/audio-config-keeper-mute.json")
	cardConf, portConf := ck.GetCardAndPortConfig("test-card", "test-port")
	assert.NotNil(t, cardConf)
	assert.NotNil(t, portConf)
}

func Test_SetEnabled(t *testing.T) {
	ck := NewConfigKeeper("testdata/audio-config-keeper.json", "testdata/audio-config-keeper-mute.json")

	ck.SetEnabled("test-card", "test-port", true)
	_, portConf := ck.GetCardAndPortConfig("test-card", "test-port")
	assert.True(t, portConf.Enabled)

	ck.SetEnabled("test-card", "test-port", false)
	_, portConf = ck.GetCardAndPortConfig("test-card", "test-port")
	assert.False(t, portConf.Enabled)
}

func Test_SetVolume(t *testing.T) {
	ck := NewConfigKeeper("testdata/audio-config-keeper.json", "testdata/audio-config-keeper-mute.json")

	ck.SetVolume("test-card", "test-port", 0.5)
	_, portConf := ck.GetCardAndPortConfig("test-card", "test-port")
	assert.Equal(t, 0.5, portConf.Volume)

	ck.SetVolume("test-card", "test-port", 0.8)
	_, portConf = ck.GetCardAndPortConfig("test-card", "test-port")
	assert.Equal(t, 0.8, portConf.Volume)
}

func Test_SetIncreaseVolume(t *testing.T) {
	ck := NewConfigKeeper("testdata/audio-config-keeper.json", "testdata/audio-config-keeper-mute.json")

	ck.SetIncreaseVolume("test-card", "test-port", true)
	_, portConf := ck.GetCardAndPortConfig("test-card", "test-port")
	assert.True(t, portConf.IncreaseVolume)

	ck.SetIncreaseVolume("test-card", "test-port", false)
	_, portConf = ck.GetCardAndPortConfig("test-card", "test-port")
	assert.False(t, portConf.IncreaseVolume)
}

func Test_SetBalance(t *testing.T) {
	ck := NewConfigKeeper("testdata/audio-config-keeper.json", "testdata/audio-config-keeper-mute.json")

	ck.SetBalance("test-card", "test-port", 0.5)
	_, portConf := ck.GetCardAndPortConfig("test-card", "test-port")
	assert.Equal(t, 0.5, portConf.Balance)

	ck.SetBalance("test-card", "test-port", -0.5)
	_, portConf = ck.GetCardAndPortConfig("test-card", "test-port")
	assert.Equal(t, -0.5, portConf.Balance)
}

func Test_SetReduceNoise(t *testing.T) {
	ck := NewConfigKeeper("testdata/audio-config-keeper.json", "testdata/audio-config-keeper-mute.json")

	ck.SetReduceNoise("test-card", "test-port", true)
	_, portConf := ck.GetCardAndPortConfig("test-card", "test-port")
	assert.True(t, portConf.ReduceNoise)

	ck.SetReduceNoise("test-card", "test-port", false)
	_, portConf = ck.GetCardAndPortConfig("test-card", "test-port")
	assert.False(t, portConf.ReduceNoise)
}

func Test_SetMuteOutput(t *testing.T) {
	ck := NewConfigKeeper("testdata/audio-config-keeper.json", "testdata/audio-config-keeper-mute.json")

	ck.SetMuteOutput(true)
	assert.True(t, ck.Mute.MuteOutput)

	ck.SetMuteOutput(false)
	assert.False(t, ck.Mute.MuteOutput)
}

func Test_SetMuteInput(t *testing.T) {
	ck := NewConfigKeeper("testdata/audio-config-keeper.json", "testdata/audio-config-keeper-mute.json")

	ck.SetMuteInput(true)
	assert.True(t, ck.Mute.MuteInput)

	ck.SetMuteInput(false)
	assert.False(t, ck.Mute.MuteInput)
}
