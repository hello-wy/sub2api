package handler

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeOpenAISpeechBody(t *testing.T) {
	got, err := normalizeOpenAISpeechBody([]byte(`{"model":"tts-1","input":"你好","voice":"ara","response_format":"wav","speed":1.2}`))
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(got, &payload))
	require.Equal(t, "你好", payload["text"])
	require.Equal(t, "ara", payload["voice_id"])
	require.Equal(t, "auto", payload["language"])
	require.Equal(t, 1.2, payload["speed"])
	outputFormat, ok := payload["output_format"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "wav", outputFormat["codec"])
}

func TestNormalizeOpenAISpeechBodyRejectsUnsupportedOptions(t *testing.T) {
	_, err := normalizeOpenAISpeechBody([]byte(`{"input":"hello","response_format":"opus"}`))
	require.EqualError(t, err, `response_format "opus" is not supported by Grok TTS`)

	_, err = normalizeOpenAISpeechBody([]byte(`{"input":"hello","instructions":"whisper"}`))
	require.EqualError(t, err, "instructions is not supported by Grok TTS")
}
