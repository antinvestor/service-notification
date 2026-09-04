// Package templates is the registration contract for message templates.
//
// A consumer service declares its templates once, in code, and calls Sync
// from its setup job on every deploy. TemplateSave on the notification
// service is an upsert keyed by (tenant, partition, name), so the sync is
// idempotent and a changed body simply overwrites the previous one.
//
// Conventions:
//   - Name: template.<service>.<entity>.<event>, lowercase, dot separated.
//   - Bodies: keyed by channel type as the notification service routes them
//     ("sms", "email"); bodies are Go text/template with {{.variable}} keys.
//   - Subject: applies to channels that carry one (email); it travels in the
//     reserved "subject" key of the save request.
//   - Variables: documents the payload keys the bodies use, so tests can
//     render every template and sends can be validated.
package templates

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"buf.build/gen/go/antinvestor/notification/connectrpc/go/notification/v1/notificationv1connect"
	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	ChannelSMS   = "sms"
	ChannelEmail = "email"

	// SubjectKey is the reserved data key that carries the subject.
	SubjectKey = "subject"

	DefaultLanguage = "en"
)

var namePattern = regexp.MustCompile(`^template\.[a-z0-9_]+(\.[a-z0-9_]+)+$`)

// Template is one message in every channel it is delivered on.
type Template struct {
	Name      string
	Language  string
	Subject   string
	Bodies    map[string]string
	Variables []string
	// Extra is stored alongside the template (owner, notes); the "variables"
	// and "owner" keys are filled by Sync when absent.
	Extra map[string]any
}

// Saver is the slice of the notification client Sync needs.
type Saver interface {
	TemplateSave(ctx context.Context, req *connect.Request[notificationv1.TemplateSaveRequest]) (*connect.Response[notificationv1.TemplateSaveResponse], error)
}

var _ Saver = notificationv1connect.NotificationServiceClient(nil)

// Validate checks the template is well formed and every body parses.
func (t Template) Validate() error {
	if !namePattern.MatchString(t.Name) {
		return fmt.Errorf("template name %q must look like template.<service>.<entity>.<event>", t.Name)
	}
	if len(t.Bodies) == 0 {
		return fmt.Errorf("template %s has no bodies", t.Name)
	}
	for channel, body := range t.Bodies {
		if channel == "" || channel == SubjectKey || len(channel) > 10 {
			return fmt.Errorf("template %s: channel %q is not a valid channel type", t.Name, channel)
		}
		if strings.TrimSpace(body) == "" {
			return fmt.Errorf("template %s: %s body is empty", t.Name, channel)
		}
		if _, err := template.New(t.Name + "/" + channel).Parse(body); err != nil {
			return fmt.Errorf("template %s: %s body: %w", t.Name, channel, err)
		}
	}
	if t.Subject != "" {
		if _, err := template.New(t.Name + "/subject").Parse(t.Subject); err != nil {
			return fmt.Errorf("template %s: subject: %w", t.Name, err)
		}
	}
	return nil
}

// SaveRequest builds the upsert request for the template.
func (t Template) SaveRequest(owner string) (*notificationv1.TemplateSaveRequest, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}
	data := make(map[string]any, len(t.Bodies)+1)
	for channel, body := range t.Bodies {
		data[channel] = body
	}
	if t.Subject != "" {
		data[SubjectKey] = t.Subject
	}
	dataStruct, err := structpb.NewStruct(data)
	if err != nil {
		return nil, err
	}
	extra := make(map[string]any, len(t.Extra)+2)
	for k, v := range t.Extra {
		extra[k] = v
	}
	if _, ok := extra["owner"]; !ok && owner != "" {
		extra["owner"] = owner
	}
	if _, ok := extra["variables"]; !ok {
		vars := make([]any, 0, len(t.Variables))
		for _, v := range t.Variables {
			vars = append(vars, v)
		}
		extra["variables"] = vars
	}
	extraStruct, err := structpb.NewStruct(extra)
	if err != nil {
		return nil, err
	}
	lang := t.Language
	if lang == "" {
		lang = DefaultLanguage
	}
	return notificationv1.TemplateSaveRequest_builder{Name: t.Name, LanguageCode: lang, Data: dataStruct, Extra: extraStruct}.Build(), nil
}

// Sync registers every template with the notification service. It returns
// the number registered before the first failure. owner names the calling
// service (e.g. "service_imports") and is recorded in each template's extra.
func Sync(ctx context.Context, cli Saver, owner string, templates []Template) (int, error) {
	seen := make(map[string]struct{}, len(templates))
	for i, t := range templates {
		key := t.Name + "/" + t.Language
		if _, dup := seen[key]; dup {
			return i, fmt.Errorf("template %s (%s) declared twice", t.Name, t.Language)
		}
		seen[key] = struct{}{}
		req, err := t.SaveRequest(owner)
		if err != nil {
			return i, err
		}
		if _, err := cli.TemplateSave(ctx, connect.NewRequest(req)); err != nil {
			return i, fmt.Errorf("register template %s: %w", t.Name, err)
		}
	}
	return len(templates), nil
}
