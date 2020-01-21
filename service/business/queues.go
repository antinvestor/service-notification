package business

import (
	"antinvestor.com/service/notification/service/repository"
	"antinvestor.com/service/notification/service/repository/models"
	"antinvestor.com/service/notification/utils"
	"context"
	"fmt"
	"github.com/nats-io/stan.go"
	"strconv"
	"time"
)

type QueueSubscriptionManager struct {
	Env               *utils.Env
	channelRepository repository.ChannelRepository

	messageOutLoggedSubscription      stan.Subscription
	messageOutChannelledSubscriptions map[string]stan.Subscription

	messageInLoggedSubscription      stan.Subscription
	messageInChannelledSubscriptions map[string]stan.Subscription
}

func (qMan *QueueSubscriptionManager) Refresh(ctx context.Context) error {
	return qMan.initiateSubscriptions(ctx)
}

func (qMan *QueueSubscriptionManager) Init(ctx context.Context) error {
	return qMan.initiateSubscriptions(ctx)
}

func (qMan *QueueSubscriptionManager) Close(ctx context.Context) {

	err := qMan.messageOutLoggedSubscription.Close()
	if err != nil {
		qMan.Env.Logger.WithError(err).Info("Problem closing message out logged subscription")
	}

	for _, subscription := range qMan.messageOutChannelledSubscriptions {
		err = subscription.Close()
		if err != nil {
			qMan.Env.Logger.WithError(err).Info("Problem closing message out channel subscription")
		}
	}

	err = qMan.messageInLoggedSubscription.Close()
	if err != nil {
		qMan.Env.Logger.WithError(err).Info("Problem closing message in logged subscription")
	}

	for _, subscription := range qMan.messageInChannelledSubscriptions {
		err = subscription.Close()
		if err != nil {
			qMan.Env.Logger.WithError(err).Info("Problem closing message in channel subscription")
		}
	}

}

func (qMan *QueueSubscriptionManager) subscribe(queueName string, handler func(m *stan.Msg), ackWaitTime time.Duration, maxInFlightMessages int) (stan.Subscription, error) {

	return qMan.Env.Queue.QueueSubscribe(
		queueName, queueName, handler, stan.DurableName(utils.ConfigQueuesDurableName),
		stan.StartWithLastReceived(), stan.SetManualAckMode(), stan.AckWait(ackWaitTime),
		stan.MaxInflight(maxInFlightMessages))
}

func (qMan *QueueSubscriptionManager) initiateSubscriptions(ctx context.Context) error {

	qMan.channelRepository = repository.NewChannelRepository(ctx, qMan.Env)

	mimConf := utils.GetEnv(utils.EnvQueueMaximumInflightMessages, "50")
	maxInflightMessages, err := strconv.Atoi(mimConf)
	if err != nil {
		maxInflightMessages = 50
	}
	ackWaitTimeConfig := utils.GetEnv(utils.EnvQueueAcknowledgementWaitTime, "300s")
	ackWaitTime, _ := time.ParseDuration(ackWaitTimeConfig)

	if qMan.messageOutLoggedSubscription == nil || !qMan.messageOutLoggedSubscription.IsValid() {

		if qMan.messageOutLoggedSubscription != nil && !qMan.messageOutLoggedSubscription.IsValid() {
			err = qMan.messageOutLoggedSubscription.Close()
			if err != nil {
				qMan.Env.Logger.WithError(err).Warn("could not close invalid message out subscription")
			}
		}

		qMan.messageOutLoggedSubscription, err = qMan.subscribe(utils.ConfigQueueMessageOutLoggedName,
			MessageOutLoggedQueueHandler(qMan.Env), ackWaitTime, maxInflightMessages)

		if err != nil {
			return err
		}
	}

	messageOutChannels, err := qMan.channelRepository.GetByMode(models.ChannelModeTransmit)
	if err != nil {
		return err
	}

	if qMan.messageOutChannelledSubscriptions == nil {
		qMan.messageOutChannelledSubscriptions = make(map[string]stan.Subscription)
	}

	msgOutChannelsMap := make(map[string]stan.Subscription)

	for _, channel := range messageOutChannels {

		subscription, exist := qMan.messageOutChannelledSubscriptions[channel.ChannelID]
		if exist {
			if !subscription.IsValid() {
				err = subscription.Close()
				if err != nil {
					qMan.Env.Logger.WithError(err).Warn("could not close invalid message out channel subscription")
				}
			} else {
				msgOutChannelsMap[channel.ChannelID] = subscription
			}

			delete(qMan.messageOutChannelledSubscriptions, channel.ChannelID)

		} else {

			queueName := fmt.Sprintf(utils.ConfigQueueMessageOutChannelledName, channel.ChannelID)

			msgOutChannelledSub, err := qMan.subscribe(queueName,
				MessageOutChanneledQueueHandler(qMan.Env), ackWaitTime, maxInflightMessages)

			if err != nil {
				return err
			}

			msgOutChannelsMap[channel.ChannelID] = msgOutChannelledSub
		}
	}

	//Drain remaining stale subscriptions
	for channelId, channel := range qMan.messageOutChannelledSubscriptions {
		err = channel.Close()
		if err != nil {
			qMan.Env.Logger.WithError(err).Warn("could not close drained message out subscription")
		}
		delete(qMan.messageOutChannelledSubscriptions, channelId)
	}

	//Add all available channels in
	qMan.messageOutChannelledSubscriptions = msgOutChannelsMap

	if qMan.messageInLoggedSubscription == nil || !qMan.messageInLoggedSubscription.IsValid() {

		if qMan.messageInLoggedSubscription != nil && !qMan.messageInLoggedSubscription.IsValid() {
			err = qMan.messageInLoggedSubscription.Close()
			if err != nil {
				qMan.Env.Logger.WithError(err).Warn("could not close invalid message in subscription")
			}
		}
		qMan.messageInLoggedSubscription, err = qMan.subscribe(utils.ConfigQueueMessageInLoggedName,
			MessageInLoggedQueueHandler(qMan.Env), ackWaitTime, maxInflightMessages)

		if err != nil {
			return err
		}
	}

	messageInChannels, err := qMan.channelRepository.GetByMode(models.ChannelModeReceive)
	if err != nil {
		return err
	}

	if qMan.messageInChannelledSubscriptions == nil {
		qMan.messageInChannelledSubscriptions = make(map[string]stan.Subscription)
	}

	msgInQueuesMap := make(map[string]stan.Subscription)

	for _, channel := range messageInChannels {

		subscription, exist := qMan.messageInChannelledSubscriptions[channel.ChannelID]
		if exist {
			if !subscription.IsValid() {
				err = subscription.Close()
				if err != nil {
					qMan.Env.Logger.WithError(err).Warn("could not close invalid message in subscription")
				}
			} else {
				msgInQueuesMap[channel.ChannelID] = subscription
			}

			delete(qMan.messageInChannelledSubscriptions, channel.ChannelID)

		} else {

			queueName := fmt.Sprintf(utils.ConfigQueueMessageInQueuedName, channel.ChannelID)
			msgInQueuedSub, err := qMan.subscribe(queueName,
				MessageInQueuedQueueHandler(qMan.Env), ackWaitTime, maxInflightMessages)

			if err != nil {
				return err
			}

			msgInQueuesMap[channel.ChannelID] = msgInQueuedSub
		}
	}

	//Drain remaining stale subscriptions
	for channelId, channel := range qMan.messageInChannelledSubscriptions {
		err = channel.Close()
		if err != nil {
			qMan.Env.Logger.WithError(err).Warn("could not close queued message in subscription")
		}
		delete(qMan.messageInChannelledSubscriptions, channelId)
	}

	qMan.messageInChannelledSubscriptions = msgInQueuesMap

	return nil
}

