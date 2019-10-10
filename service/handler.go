package service

import (
	"context"
	//"encoding/json"
	//"time"
	"bitbucket.org/antinvestor/service-notification/notification"
	//"fmt"
	//"github.com/jinzhu/gorm"
	//"log"
)



type notificationserver struct {
	Env *Env
}

func (server *notificationserver) Out(ctxt context.Context, req *notification.QueueRequest) (*notification.QueueResponse, error){

	return &notification.QueueResponse{}, nil
}	
func (server *notificationserver) Status(ctxt context.Context, req *notification.StatusRequest) (*notification.StatusResponse, error){

	return &notification.StatusResponse{}, nil
}
func (server *notificationserver) Release(ctxt context.Context, req *notification.ReleaseRequest) (*notification.StatusResponse, error){

	return &notification.StatusResponse{}, nil
}
func (server *notificationserver) In(ctxt context.Context, req *notification.IncomeRequest) (*notification.QueueResponse, error){

	return &notification.QueueResponse{}, nil
}
