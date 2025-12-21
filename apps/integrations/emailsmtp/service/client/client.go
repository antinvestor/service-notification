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
	"github.com/antinvestor/service-notification/apps/integrations/emailsmtp/config"
	"github.com/antinvestor/service-notification/internal/constants"
	"github.com/antinvestor/service-notification/internal/utility"
	"github.com/pitabwire/util"
	"github.com/wneessen/go-mail"
)

const (
	connectionTTL = 5 * time.Minute
)

type connectedClient struct {
	client      *mail.Client
	connectedAt time.Time
	mu          sync.Mutex
}

func (cc *connectedClient) isExpired() bool {
	return time.Since(cc.connectedAt) > connectionTTL
}

type Client struct {
	cfg         *config.EmailSMTPConfig
	logger      *util.LogEntry
	profileCli  profilev1connect.ProfileServiceClient
	settingsCli settingsv1connect.SettingsServiceClient
	connMap     sync.Map
	connMu      sync.Mutex
}

func NewClient(logger *util.LogEntry, cfg *config.EmailSMTPConfig, profileCli profilev1connect.ProfileServiceClient, settingsCli settingsv1connect.SettingsServiceClient) (*Client, error) {

	return &Client{
		cfg:         cfg,
		logger:      logger,
		profileCli:  profileCli,
		settingsCli: settingsCli,
		connMap:     sync.Map{},
	}, nil
}

func (ms *Client) createMailClient() (*mail.Client, error) {
	cfg := ms.cfg
	return mail.NewClient(cfg.SMTPServerHOST,
		mail.WithPort(cfg.SMTPServerPORT),
		mail.WithTLSPolicy(mail.TLSMandatory),
		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithUsername(cfg.SMTPServerAccessKey),
		mail.WithPassword(cfg.SMTPServerSecretKey),
		mail.WithTimeout(15*time.Second),
	)
}

func (ms *Client) getConnectedClient(ctx context.Context, credentials map[string]string) (*connectedClient, error) {
	partitionID := credentials[constants.PartitionIDHeaderName]

	if connObj, ok := ms.connMap.Load(partitionID); ok {
		if conn, cok := connObj.(*connectedClient); cok && !conn.isExpired() {
			return conn, nil
		}
		if conn, cok := connObj.(*connectedClient); cok {
			_ = conn.client.Close()
		}
		ms.connMap.Delete(partitionID)
	}

	ms.connMu.Lock()
	defer ms.connMu.Unlock()

	if connObj, ok := ms.connMap.Load(partitionID); ok {
		if conn, cok := connObj.(*connectedClient); cok && !conn.isExpired() {
			return conn, nil
		}
		if conn, cok := connObj.(*connectedClient); cok {
			_ = conn.client.Close()
		}
		ms.connMap.Delete(partitionID)
	}

	cli, err := ms.createMailClient()
	if err != nil {
		return nil, err
	}

	if err := cli.DialWithContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to establish SMTP connection: %w", err)
	}

	conn := &connectedClient{
		client:      cli,
		connectedAt: time.Now(),
	}
	ms.connMap.Store(partitionID, conn)

	return conn, nil
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

	// TODO: remove this hack to just see things workign
	sender.Detail = "info@stawi.im"

	extrasData := notification.GetExtras().AsMap()
	notificationSubject := ""
	if dt, ok := extrasData["subject"]; ok {
		if s, ok := dt.(string); ok {
			notificationSubject = s
		}
	}

	conn, err := ms.getConnectedClient(ctx, credentials)
	if err != nil {
		return err
	}

	err = ms.sendEmailWithRetry(ctx, credentials, conn, notification.GetId(), sender, recipient, notificationSubject, notification.GetData())
	if err != nil {
		return err
	}
	return nil
}

func (ms *Client) sendEmailWithRetry(ctx context.Context, credentials map[string]string, conn *connectedClient, messageID string, sender, recipient *commonv1.ContactLink, subject, message string) error {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	msg := mail.NewMsg()

	if sender != nil && sender.GetDetail() != "" {
		if err := msg.From(sender.GetDetail()); err != nil {
			return err
		}
	}
	if err := msg.To(recipient.GetDetail()); err != nil {
		return err
	}

	msg.SetGenHeader("X-PM-Metadata-notification-id", messageID)
	msg.Subject(subject)
	msg.SetBodyString(mail.TypeTextPlain, message)

	err := conn.client.Send(msg)
	if err != nil {
		partitionID := credentials[constants.PartitionIDHeaderName]
		ms.connMap.Delete(partitionID)
		_ = conn.client.Close()

		newConn, dialErr := ms.getConnectedClient(ctx, credentials)
		if dialErr != nil {
			return fmt.Errorf("send failed and reconnect failed: %w (original: %v)", dialErr, err)
		}

		newConn.mu.Lock()
		defer newConn.mu.Unlock()
		if retryErr := newConn.client.Send(msg); retryErr != nil {
			return fmt.Errorf("send failed after retry: %w", retryErr)
		}
	}

	return nil
}
