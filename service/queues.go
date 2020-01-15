package service

import (
	"antinvestor.com/service/notification/service/business"
	"antinvestor.com/service/notification/utils"
	"context"
)

type QueueSubscriptionManager interface {
	Init(ctx context.Context) error
	Refresh(ctx context.Context) error
	Close(ctx context.Context)
}

func NewQueueSubscriptionManager(env *utils.Env) QueueSubscriptionManager {
	return &business.QueueSubscriptionManager{Env: env}
}
