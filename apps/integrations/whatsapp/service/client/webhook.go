package client

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// SignatureHeader carries the HMAC-SHA256 of the raw webhook body, keyed by the app secret.
const SignatureHeader = "X-Hub-Signature-256"

// VerifySignature checks a webhook body against the X-Hub-Signature-256 header value.
func VerifySignature(appSecret string, body []byte, header string) bool {
	const prefix = "sha256="
	if appSecret == "" || !strings.HasPrefix(header, prefix) {
		return false
	}
	expected, err := hex.DecodeString(strings.TrimPrefix(header, prefix))
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write(body)
	return hmac.Equal(mac.Sum(nil), expected)
}

// Sign produces the header value for a body; used by tests and tooling.
func Sign(appSecret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// WebhookPayload is the top-level Cloud API webhook notification.
type WebhookPayload struct {
	Object string  `json:"object"`
	Entry  []Entry `json:"entry"`
}

type Entry struct {
	ID      string   `json:"id"`
	Changes []Change `json:"changes"`
}

type Change struct {
	Field string      `json:"field"`
	Value ChangeValue `json:"value"`
}

type ChangeValue struct {
	MessagingProduct string    `json:"messaging_product"`
	Metadata         Metadata  `json:"metadata"`
	Contacts         []Contact `json:"contacts"`
	Messages         []Message `json:"messages"`
	Statuses         []Status  `json:"statuses"`
}

type Metadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"`
}

type Contact struct {
	WaID    string `json:"wa_id"`
	Profile struct {
		Name string `json:"name"`
	} `json:"profile"`
}

// Message is an inbound user message. Only the fields the integration records are typed;
// media of any kind shares the Media shape.
type Message struct {
	From      string `json:"from"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Text      *struct {
		Body string `json:"body"`
	} `json:"text"`
	Image    *Media `json:"image"`
	Video    *Media `json:"video"`
	Audio    *Media `json:"audio"`
	Document *Media `json:"document"`
	Sticker  *Media `json:"sticker"`
	Location *struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Name      string  `json:"name"`
		Address   string  `json:"address"`
	} `json:"location"`
	Button *struct {
		Text    string `json:"text"`
		Payload string `json:"payload"`
	} `json:"button"`
	Interactive *struct {
		Type        string `json:"type"`
		ButtonReply *struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"button_reply"`
		ListReply *struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			Description string `json:"description"`
		} `json:"list_reply"`
	} `json:"interactive"`
	Context *struct {
		From string `json:"from"`
		ID   string `json:"id"`
	} `json:"context"`
	Errors []StatusError `json:"errors"`
}

type Media struct {
	ID       string `json:"id"`
	MimeType string `json:"mime_type"`
	SHA256   string `json:"sha256"`
	Caption  string `json:"caption"`
	Filename string `json:"filename"`
}

// Body returns the human-readable content of a message: text, caption, reply title,
// or a location summary. Empty for media without a caption.
func (m Message) Body() string {
	switch {
	case m.Text != nil:
		return m.Text.Body
	case m.Button != nil:
		return m.Button.Text
	case m.Interactive != nil && m.Interactive.ButtonReply != nil:
		return m.Interactive.ButtonReply.Title
	case m.Interactive != nil && m.Interactive.ListReply != nil:
		return m.Interactive.ListReply.Title
	case m.Location != nil:
		if m.Location.Name != "" {
			return m.Location.Name
		}
		return m.Location.Address
	}
	if media := m.MediaRef(); media != nil {
		return media.Caption
	}
	return ""
}

// MediaRef returns the attached media, if any.
func (m Message) MediaRef() *Media {
	for _, media := range []*Media{m.Image, m.Video, m.Audio, m.Document, m.Sticker} {
		if media != nil {
			return media
		}
	}
	return nil
}

// ReplyPayload returns the machine payload of a button or list reply.
func (m Message) ReplyPayload() string {
	switch {
	case m.Button != nil:
		return m.Button.Payload
	case m.Interactive != nil && m.Interactive.ButtonReply != nil:
		return m.Interactive.ButtonReply.ID
	case m.Interactive != nil && m.Interactive.ListReply != nil:
		return m.Interactive.ListReply.ID
	}
	return ""
}

// Status is a delivery status update for a message the business sent.
type Status struct {
	ID           string        `json:"id"`
	Status       string        `json:"status"`
	Timestamp    string        `json:"timestamp"`
	RecipientID  string        `json:"recipient_id"`
	Errors       []StatusError `json:"errors"`
	Conversation *struct {
		ID     string `json:"id"`
		Origin struct {
			Type string `json:"type"`
		} `json:"origin"`
	} `json:"conversation"`
	Pricing *struct {
		Billable     bool   `json:"billable"`
		PricingModel string `json:"pricing_model"`
		Category     string `json:"category"`
	} `json:"pricing"`
}

type StatusError struct {
	Code      int    `json:"code"`
	Title     string `json:"title"`
	Message   string `json:"message"`
	ErrorData struct {
		Details string `json:"details"`
	} `json:"error_data"`
}
