package client

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVerifySignature(t *testing.T) {
	t.Parallel()

	body := []byte(`{"object":"whatsapp_business_account"}`)
	secret := "s3cr3t"

	require.True(t, VerifySignature(secret, body, Sign(secret, body)))
	require.False(t, VerifySignature(secret, []byte(`{"object":"tampered"}`), Sign(secret, body)))
	require.False(t, VerifySignature("other", body, Sign(secret, body)))
	require.False(t, VerifySignature(secret, body, ""))
	require.False(t, VerifySignature(secret, body, "sha1=abcd"))
	require.False(t, VerifySignature(secret, body, "sha256=nothex"))
	require.False(t, VerifySignature("", body, Sign("", body)), "an empty secret never verifies")
}

func TestMessageBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		raw         string
		wantBody    string
		wantMedia   string
		wantPayload string
	}{
		{name: "text", raw: `{"type":"text","text":{"body":"hello"}}`, wantBody: "hello"},
		{name: "image with caption", raw: `{"type":"image","image":{"id":"m1","mime_type":"image/jpeg","caption":"pic"}}`, wantBody: "pic", wantMedia: "m1"},
		{name: "document without caption", raw: `{"type":"document","document":{"id":"d1","filename":"a.pdf"}}`, wantBody: "", wantMedia: "d1"},
		{name: "button reply", raw: `{"type":"button","button":{"text":"Yes","payload":"CONFIRM"}}`, wantBody: "Yes", wantPayload: "CONFIRM"},
		{name: "interactive list", raw: `{"type":"interactive","interactive":{"type":"list_reply","list_reply":{"id":"opt-2","title":"Option 2"}}}`, wantBody: "Option 2", wantPayload: "opt-2"},
		{name: "interactive button", raw: `{"type":"interactive","interactive":{"type":"button_reply","button_reply":{"id":"b1","title":"Go"}}}`, wantBody: "Go", wantPayload: "b1"},
		{name: "location", raw: `{"type":"location","location":{"latitude":1.2,"longitude":36.8,"address":"Nairobi"}}`, wantBody: "Nairobi"},
		{name: "unsupported", raw: `{"type":"unsupported","errors":[{"code":131051,"title":"Unsupported message type"}]}`, wantBody: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var msg Message
			require.NoError(t, json.Unmarshal([]byte(tt.raw), &msg))
			require.Equal(t, tt.wantBody, msg.Body())
			require.Equal(t, tt.wantPayload, msg.ReplyPayload())
			if tt.wantMedia == "" {
				require.Nil(t, msg.MediaRef())
			} else {
				require.Equal(t, tt.wantMedia, msg.MediaRef().ID)
			}
		})
	}
}

func TestStatusFallback(t *testing.T) {
	t.Parallel()
	require.Equal(t, FallbackChannel, StatusFallback(131026))
	require.Equal(t, FallbackChannel, StatusFallback(131047))
	require.Equal(t, "", StatusFallback(131000))
	require.Equal(t, "", StatusFallback(0))
}
