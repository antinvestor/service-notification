package client

import (
	"context"
	"fmt"
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

const (
	clientTTL       = 5 * time.Minute
	maxClientsCache = 100
)

type cachedClient struct {
	client    *mail.Client
	createdAt time.Time
}

func (cc *cachedClient) isExpired() bool {
	return time.Since(cc.createdAt) > clientTTL
}

type Client struct {
	cfg         *config.EmailSMTPConfig
	logger      *util.LogEntry
	profileCli  profilev1connect.ProfileServiceClient
	settingsCli settingsv1connect.SettingsServiceClient
	mailCliMap  sync.Map
	clientMu    sync.Mutex
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

	if clientObj, ok := ms.mailCliMap.Load(partitionID); ok {
		if cached, cok := clientObj.(*cachedClient); cok && !cached.isExpired() {
			return cached.client, nil
		}
		ms.mailCliMap.Delete(partitionID)
	}

	ms.clientMu.Lock()
	defer ms.clientMu.Unlock()

	if clientObj, ok := ms.mailCliMap.Load(partitionID); ok {
		if cached, cok := clientObj.(*cachedClient); cok && !cached.isExpired() {
			return cached.client, nil
		}
		ms.mailCliMap.Delete(partitionID)
	}

	cfg := ms.cfg

	cli, err := mail.NewClient(cfg.SMTPServerHOST,
		mail.WithPort(cfg.SMTPServerPORT),
		mail.WithTLSPolicy(mail.TLSMandatory),
		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithUsername(cfg.SMTPServerAccessKey),
		mail.WithPassword(cfg.SMTPServerSecretKey),
		mail.WithTimeout(15*time.Second),
		mail.WithTLSConfig(nil),
	)
	if err != nil {
		return nil, err
	}

	cached := &cachedClient{
		client:    cli,
		createdAt: time.Now(),
	}
	ms.mailCliMap.Store(partitionID, cached)

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
	if dt, ok := extrasData["subject"]; ok {
		if s, ok := dt.(string); ok {
			notificationSubject = s
		}
	}

	cli, err := ms.getMailClient(ctx, credentials)
	if err != nil {
		return err
	}
	if cli == nil {
		return fmt.Errorf("failed to get mail client")
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
