package client

import (
	"context"
	"fmt"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	profilev1 "buf.build/gen/go/antinvestor/profile/protocolbuffers/go/profile/v1"
	"connectrpc.com/connect"
	"github.com/antinvestor/service-notification/apps/integrations/emailsmtp/config"
	"github.com/pitabwire/util"
	"github.com/wneessen/go-mail"
)

type Client struct {
	cfg        *config.EmailSMTPConfig
	logger     *util.LogEntry
	profileCli profilev1connect.ProfileServiceClient
	mailCli    *mail.Client
}

func NewClient(logger *util.LogEntry, cfg *config.EmailSMTPConfig, profileCli profilev1connect.ProfileServiceClient) (*Client, error) {

	cli, err := mail.NewClient(cfg.SMTPServerHOST,
		mail.WithPort(cfg.SMTPServerPORT), mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithUsername(cfg.SMTPServerUserName),
		mail.WithPassword(cfg.SMTPServerPassword),
		mail.WithTLSPolicy(mail.TLSOpportunistic))
	if err != nil {
		return nil, err
	}

	return &Client{
		cfg:        cfg,
		logger:     logger,
		profileCli: profileCli,
		mailCli:    cli,
	}, nil
}

func (ms *Client) contactLinkToEmail(ctx context.Context, contact *commonv1.ContactLink) (string, error) {

	if contact.GetDetail() != "" {
		return contact.GetDetail(), nil
	}

	result, err := ms.profileCli.GetByContact(ctx, connect.NewRequest(&profilev1.GetByContactRequest{Contact: contact.GetContactId()}))
	if err != nil {
		return "", err
	}

	profile := result.Msg.GetData()

	for _, c := range profile.GetContacts() {
		if c.GetType() == profilev1.ContactType_EMAIL {
			if c.GetId() == contact.GetContactId() {
				return c.GetDetail(), nil

			}
		}
	}

	return "", fmt.Errorf("no valid contact exists in request")
}

func (ms *Client) Send(ctx context.Context, _ map[string]string, notification *notificationv1.Notification) error {

	recipientEmail, err := ms.contactLinkToEmail(ctx, notification.GetRecipient())
	if err != nil {
		return err
	}

	senderEmail, err := ms.contactLinkToEmail(ctx, notification.GetSource())
	if err != nil {
		return err
	}

	extrasData := notification.GetExtras().AsMap()
	notificationSubject := ""
	dt, ok := extrasData["subject"]
	if ok {
		notificationSubject = dt.(string)
	}

	err = ms.SendEmail(ctx, notification.GetId(), senderEmail, recipientEmail, notificationSubject, notification.GetData())
	if err != nil {
		return err
	}
	return nil
}

// SendEmail immediately sends out messages using the configured settings.
func (ms *Client) SendEmail(ctx context.Context, messageID, senderEmail, recipientEmail string, subject string, message string) error {

	msg := mail.NewMsg()
	err := msg.From(senderEmail)
	if err != nil {
		return err
	}
	err = msg.To(recipientEmail)
	if err != nil {
		return err
	}
	err = msg.SetAddrHeader("X-PM-Metadata-notification-id", messageID)
	if err != nil {
		return err
	}
	msg.Subject(subject)
	msg.SetBodyString(mail.TypeTextPlain, message)

	err = ms.mailCli.DialAndSendWithContext(ctx, msg)
	if err != nil {
		return err
	}

	return nil
}
