package business

import (
	"antinvestor.com/service/notification/grpc/profile"
	"antinvestor.com/service/notification/service/repository"
	"antinvestor.com/service/notification/service/repository/models"
	"antinvestor.com/service/notification/utils"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/nats-io/stan.go"
	"text/template"
)

func routeOutboundNotification(channelRepo repository.ChannelRepository, notification *models.Notification) error {

	channels, err := channelRepo.GetByModeAndTypeAndProductID(models.ChannelModeTransmit, notification.Type, notification.ProductID)
	if err != nil {
		return err
	}

	if len(channels) > 0 {
		channel := channels[0]
		if len(channels) > 1 {
			//TODO: find a simple way of routing message mostly by settings
			// or contact and profile preferences
		}
		notification.ChannelID = channel.ChannelID
	} else {
		return errors.New(fmt.Sprintf("No channels matched for routing message out : %s", notification.ProductID))
	}

	return nil
}

func formatOutboundNotification(templateRepository repository.TemplateRepository, notification *models.Notification) (map[string]string, error) {

	var templateMap map[string]string
	tmplDetail, err := templateRepository.GetByNameProductIDAndLanguageID(notification.TemplateID,
		notification.ProductID, notification.LanguageID)
	if err != nil {
		return nil, err
	}

	payload := make(map[string]string)
	err = json.Unmarshal(notification.Payload.RawMessage, payload)

	for _, data := range tmplDetail.DataList {

		tmpl, err := template.New("message_out").Parse(data.Detail)
		if err != nil {
			return nil, err
		}

		var tmplBytes bytes.Buffer
		err = tmpl.Execute(&tmplBytes, payload)
		templateMap[data.Type] = tmplBytes.String()

	}

	return templateMap, nil
}

func getContactFromProfileByContactID(profile *profile.ProfileObject, contactID string) *profile.ContactObject {

	for _, contact := range profile.GetContacts() {
		if contact.GetID() == contactID {
			return contact
		}
	}

	return nil
}

func pushOutMessage(ctx context.Context, env *utils.Env, profile *profile.ProfileObject, contact *profile.ContactObject, notification *models.Notification, templateMap map[string]string) (map[string]string, error) {

	env.Logger.Info("===========================================================")
	env.Logger.Info("We have successfully managed to get to post out ")
	env.Logger.Infof("Contact details : %s", contact.Detail)
	env.Logger.Infof("Notification details : %s", notification.NotificationID)
	env.Logger.Infof("Message details : %s", templateMap)
	env.Logger.Info("===========================================================")
	responseMap := make(map[string]string)
	responseMap["message"] = templateMap["text"]
	responseMap["external_id"] = ""
	responseMap["transient_id"] = ""
	responseMap["state"] = ""
	responseMap["extra"] = ""
	return responseMap, nil
}

// MessageOutLoggedQueueHandler Method subscribing to receive all messages that will be queued after being logged on their way out of the system.
func MessageOutLoggedQueueHandler(env *utils.Env) func(*stan.Msg) {

	return func(msg *stan.Msg) {

		go func() {

			notificationId, ctx, span, err := utils.QueueGetID(msg, "Queue message out logged")
			if err != nil {
				env.Logger.WithError(err).Infof("Unable extract queue id and span from message with sequence %d", msg.Sequence)
				err = msg.Ack()
				if err != nil {
					env.Logger.WithError(err).Infof("Unable to ack message with sequence %d", msg.Sequence)
				}
				return
			}

			defer span.Finish()

			notificationRepo := repository.NewNotificationRepository(ctx, env)
			channelRepo := repository.NewChannelRepository(ctx, env)

			n, err := notificationRepo.GetByID(notificationId)
			if err != nil {
				env.Logger.WithError(err).Warnf("Unable to obtain an outbound notification by queue id %d", msg.Sequence)
				return
			}

			p, err := profile.GetProfileByID(ctx, env.GetProfileServiceConn(), n.ProfileID)
			if err != nil {
				env.Logger.WithError(err).Infof("Could not get the profile with id : %s", n.ProfileID)
				return
			}

			contact := getContactFromProfileByContactID(p, n.ContactID)
			switch contact.Type {
			case profile.ContactType_PHONE:
				n.Type = "sms"
			case profile.ContactType_EMAIL:
				n.Type = "email"
			default:
				n.Type = "unknown"
			}

			err = routeOutboundNotification(channelRepo, n)
			if err != nil {
				env.Logger.WithError(err).Errorf("Unable to route outbound notification by id %s", n.NotificationID)
				return
			}

			err = notificationRepo.Save(n)
			if err != nil {
				env.Logger.WithError(err).Warnf("Unable to update outbound notification by id %s", n.NotificationID)
				return
			}

			queueID, err := utils.QueueMakeID(ctx, n.NotificationID)
			queueName := fmt.Sprintf(utils.ConfigQueueMessageOutChannelledName, n.ChannelID)
			// Queue out a routed message for further processing
			err = env.Queue.Publish(queueName, queueID)
			if err != nil {
				env.Logger.WithError(err).Errorf("Could not publish channeled message out with id : %s", n.NotificationID)
				return
			}

			err = msg.Ack()
			if err != nil {
				env.Logger.WithError(err).Infof("Unable to ack message with sequence %d", msg.Sequence)
			}

		}()
	}
}

func MessageOutChanneledQueueHandler(env *utils.Env) func(*stan.Msg) {

	return func(msg *stan.Msg) {
		go func() {

			notificationId, ctx, span, err := utils.QueueGetID(msg, "Queue message out channel")
			if err != nil {
				env.Logger.WithError(err).Infof("Unable extract queue id and span from message with sequence %d", msg.Sequence)
				err = msg.Ack()
				if err != nil {
					env.Logger.WithError(err).Infof("Unable to ack message with sequence %d", msg.Sequence)
				}
				return
			}

			defer span.Finish()

			notificationRepo := repository.NewNotificationRepository(ctx, env)
			templateRepo := repository.NewTemplateRepository(ctx, env)

			n, err := notificationRepo.GetByID(notificationId)
			if err != nil {
				env.Logger.WithError(err).Infof("Unable to obtain an outbound notification by queue id %d", msg.Sequence)
				return
			}

			p, err := profile.GetProfileByID(ctx, env.GetProfileServiceConn(), n.ProfileID)
			if err != nil {
				env.Logger.WithError(err).Infof("Could not get the p with id : %s", n.ProfileID)
				return
			}

			contact := getContactFromProfileByContactID(p, n.ContactID)

			templateMap, err := formatOutboundNotification(templateRepo, n)
			if err != nil {
				env.Logger.WithError(err).Errorf("Unable to format outbound notification by id %s", n.NotificationID)
				return
			}

			response, err := pushOutMessage(ctx, env, p, contact, n, templateMap)
			if err != nil {
				env.Logger.WithError(err).Infof("Could not push out message with id : %s", n.NotificationID)
				return
			}

			n.Message = response["message"]
			n.ExternalID = response["external_id"]
			n.TransientID = response["transient_id"]
			n.State = response["state"]
			n.Extra = response["extra"]

			err = notificationRepo.Save(n)
			if err != nil {
				env.Logger.WithError(err).Infof("Unable to update message with id :%s", n.NotificationID)
				return
			}

			err = msg.Ack()
			if err != nil {
				env.Logger.WithError(err).Infof("Unable to ack message with sequence %d", msg.Sequence)
			}

		}()
	}
}

func MessageInLoggedQueueHandler(env *utils.Env) func(*stan.Msg) {

	return func(msg *stan.Msg) {

		go func() {
			notificationId, ctx, span, err := utils.QueueGetID(msg, "Queue message in logged")
			if err != nil {
				env.Logger.WithError(err).Infof("Unable extract queue id and span from message with sequence %d", msg.Sequence)
				err = msg.Ack()
				if err != nil {
					env.Logger.WithError(err).Infof("Unable to ack message with sequence %d", msg.Sequence)
				}
				return
			}

			defer span.Finish()

			notificationRepo := repository.NewNotificationRepository(ctx, env)

			_, err = notificationRepo.GetByID(notificationId)
			if err != nil {
				env.Logger.WithError(err).Infof("Unable to obtain an inbound notification by queue id %d", msg.Sequence)
				return
			}

		}()
	}
}

func MessageInQueuedQueueHandler(env *utils.Env) func(*stan.Msg) {

	return func(msg *stan.Msg) {
		go func() {

			notificationId, ctx, span, err := utils.QueueGetID(msg, "Queue message out channel")
			if err != nil {
				env.Logger.WithError(err).Infof("Unable extract queue id and span from message with sequence %d", msg.Sequence)
				err = msg.Ack()
				if err != nil {
					env.Logger.WithError(err).Infof("Unable to ack message with sequence %d", msg.Sequence)
				}
				return
			}

			defer span.Finish()

			notificationRepo := repository.NewNotificationRepository(ctx, env)

			_, err = notificationRepo.GetByID(notificationId)
			if err != nil {
				env.Logger.WithError(err).Infof("Unable to obtain an outbound notification by queue id %d", msg.Sequence)
				return
			}

		}()
	}
}
