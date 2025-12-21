package client

import (
	"context"
	"sync"
	"time"

	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	profilev1 "buf.build/gen/go/antinvestor/profile/protocolbuffers/go/profile/v1"
	"buf.build/gen/go/antinvestor/settingz/connectrpc/go/settings/v1/settingsv1connect"
	"github.com/antinvestor/service-notification/apps/integrations/emailsmtp/config"
	"github.com/antinvestor/service-notification/internal/constants"
	"github.com/antinvestor/service-notification/internal/utility"
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


func (ms *Client) Send(ctx context.Context, credentials map[string]string, notification *notificationv1.Notification) error {

	recipient, err := utility.PopulateContactLink(ctx, ms.profileCli, notification.GetRecipient(), profilev1.ContactType_EMAIL)
	if err != nil {
		return err
	}

	sender, err := utility.PopulateContactLink(ctx, ms.profileCli, notification.GetSource(), profilev1.ContactType_EMAIL)
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

	err = ms.SendEmail(ctx, cli, notification.GetId(), sender.GetDetail(), recipient.GetDetail(), notificationSubject, notification.GetData())
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
