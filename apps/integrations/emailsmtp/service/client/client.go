package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	profilev1 "buf.build/gen/go/antinvestor/profile/protocolbuffers/go/profile/v1"
	"buf.build/gen/go/antinvestor/settingz/connectrpc/go/settings/v1/settingsv1connect"
	"connectrpc.com/connect"
	"github.com/antinvestor/service-notification/apps/integrations/emailsmtp/config"
	"github.com/antinvestor/service-notification/internal/constants"
	"github.com/pitabwire/util"
	"github.com/wneessen/go-mail"
)

type Client struct {
	cfg         *config.EmailSMTPConfig
	logger      *util.LogEntry
	profileCli  profilev1connect.ProfileServiceClient
	settingsCli settingsv1connect.SettingsServiceClient
	mailCliMap  sync.Map
}

func NewClient(logger *util.LogEntry, cfg *config.EmailSMTPConfig, profileCli profilev1connect.ProfileServiceClient, settingsCli settingsv1connect.SettingsServiceClient) (*Client, error) {

	return &Client{
		cfg:         cfg,
		logger:      logger,
		profileCli:  profileCli,
		settingsCli: settingsCli,
		mailCliMap:  sync.Map{},
	}, nil
}

func (ms *Client) getMailClient(_ context.Context, credentials map[string]string) (*mail.Client, error) {
	partitionID := credentials[constants.PartitionIDHeaderName]
	clientObj, ok := ms.mailCliMap.Load(partitionID)
	if ok {
		cli, cok := clientObj.(*mail.Client)
		if cok {
			return cli, nil
		}
	}

	// settings, err := ms.settingsCli.Get(ctx, connect.NewRequest(&settingsv1.GetRequest{Key: partitionID}))
	// if err != nil {
	// 	return nil, err
	// }

	cfg := ms.cfg

	cli, err := mail.NewClient(cfg.SMTPServerHOST,
		mail.WithPort(cfg.SMTPServerPORT),

		// Enforce STARTTLS upgrade
		mail.WithTLSPolicy(mail.TLSMandatory),

		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithUsername(cfg.SMTPServerAccessKey),
		mail.WithPassword(cfg.SMTPServerSecretKey),

		// Reliability
		mail.WithTimeout(15*time.Second),

		// Use system CAs
		mail.WithTLSConfig(nil),
	)
	if err != nil {
		return nil, err
	}

	ms.mailCliMap.Store(partitionID, cli)

	return cli, nil

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

func (ms *Client) Send(ctx context.Context, credentials map[string]string, notification *notificationv1.Notification) error {

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

	cli, err := ms.getMailClient(ctx, credentials)
	if cli == nil {
		return err
	}

	err = ms.SendEmail(ctx, cli, notification.GetId(), senderEmail, recipientEmail, notificationSubject, notification.GetData())
	if err != nil {
		return err
	}
	return nil
}

// SendEmail immediately sends out messages using the configured settings.
func (ms *Client) SendEmail(ctx context.Context, cli *mail.Client, messageID, senderEmail, recipientEmail string, subject string, message string) error {

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

	err = cli.DialAndSendWithContext(ctx, msg)
	if err != nil {
		return err
	}

	return nil
}
