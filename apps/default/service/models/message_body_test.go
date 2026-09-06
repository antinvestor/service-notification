package models

import (
	"context"
	"testing"
)

func TestSelectMessageBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		notificationType string
		message          map[string]string
		want             string
	}{
		{name: "exact type wins", notificationType: RouteTypeWhatsAppForm,
			message: map[string]string{"whatsapp": "wa body", "sms": "sms body", "text": "text body"}, want: "wa body"},
		{name: "falls back to text", notificationType: RouteTypeWhatsAppForm,
			message: map[string]string{"text": "text body", "sms": "sms body"}, want: "text body"},
		{name: "falls back to default", notificationType: RouteTypeSMSForm,
			message: map[string]string{"default": "default body", "email": "email body"}, want: "default body"},
		{name: "falls back to sms", notificationType: RouteTypeWhatsAppForm,
			message: map[string]string{"sms": "sms body", "email": "email body"}, want: "sms body"},
		{name: "falls back to only remaining channel body", notificationType: RouteTypeSMSForm,
			message: map[string]string{"email": "email body"}, want: "email body"},
		{name: "ignores non-body keys", notificationType: RouteTypeSMSForm,
			message: map[string]string{"subject": "subj", "support_email": "help@x", "whatsapp_template": "{}"}, want: ""},
		{name: "empty map", notificationType: RouteTypeSMSForm, message: nil, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := SelectMessageBody(tt.notificationType, tt.message); got != tt.want {
				t.Fatalf("SelectMessageBody() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNotificationToAPIToleratesNilRelations(t *testing.T) {
	t.Parallel()

	n := &Notification{NotificationType: RouteTypeSMSForm, Message: "hi"}
	n.ID = "notif-1"

	api := n.ToAPI(nil, nil, nil)
	if api.GetId() != "notif-1" || api.GetData() != "hi" {
		t.Fatalf("unexpected conversion: %v", api)
	}
	if api.GetStatus() != nil {
		t.Fatalf("expected nil status, got %v", api.GetStatus())
	}
	if api.GetLanguage() != "" {
		t.Fatalf("expected empty language, got %q", api.GetLanguage())
	}
}

func TestNewFallbackNotification(t *testing.T) {
	t.Parallel()

	parent := &Notification{
		RecipientContactID: "contact-1",
		RecipientProfileID: "profile-1",
		SenderProfileID:    "sender-1",
		LanguageID:         "lang-1",
		TemplateID:         "tmpl-1",
		NotificationType:   RouteTypeWhatsAppForm,
		RouteID:            "route-wa",
		Message:            "hello",
		Payload:            map[string]any{"code": "1234"},
		Priority:           1,
		ExternalID:         "wamid.1",
	}
	parent.ID = "parent-1"
	parent.TenantID = "t1"
	parent.PartitionID = "p1"
	parent.AccessID = "a1"

	child := NewFallbackNotification(context.Background(), parent, RouteTypeSMSForm)

	if child.GetID() == "" || child.GetID() == parent.GetID() {
		t.Fatalf("child must get its own id, got %q", child.GetID())
	}
	if again := NewFallbackNotification(context.Background(), parent, RouteTypeSMSForm); again.GetID() != child.GetID() {
		t.Fatalf("fallback id must be deterministic per parent: %q vs %q", again.GetID(), child.GetID())
	}
	if child.ParentID != "parent-1" || child.NotificationType != RouteTypeSMSForm {
		t.Fatalf("child lineage/type wrong: %+v", child)
	}
	if child.RouteID != "" || child.ExternalID != "" {
		t.Fatalf("child must not inherit route or external id: %+v", child)
	}
	if !child.IsReleased() || !child.OutBound {
		t.Fatalf("child must be released and outbound: %+v", child)
	}
	if child.TenantID != "t1" || child.PartitionID != "p1" || child.AccessID != "a1" {
		t.Fatalf("child must keep partition scope: %+v", child)
	}
	if child.RecipientContactID != "contact-1" || child.LanguageID != "lang-1" || child.TemplateID != "tmpl-1" || child.Message != "hello" {
		t.Fatalf("child must copy content: %+v", child)
	}
	child.Payload["code"] = "changed"
	if parent.Payload["code"] != "1234" {
		t.Fatal("child payload must be a copy, not shared with the parent")
	}
}
