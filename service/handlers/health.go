package handlers

import (
	grpc_health_v1 "antinvestor.com/service/notification/grpc/health"
	"antinvestor.com/service/notification/utils"
	"context"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (server *Notificationserver) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {

	span, _ := opentracing.StartSpanFromContext(ctx, "HealthCheckEndpoint")
	defer span.Finish()

	statusCode, _ := utils.HealthCheckProcessing(server.Env.Logger, server.Env.Health)

	grpcStatus := grpc_health_v1.HealthCheckResponse_NOT_SERVING
	switch statusCode {
	case 200:
		grpcStatus = grpc_health_v1.HealthCheckResponse_SERVING
		break
	case 500:
		grpcStatus = grpc_health_v1.HealthCheckResponse_UNKNOWN
		break
	}
	grpcResponse := grpc_health_v1.HealthCheckResponse{
		Status: grpcStatus,
	}

	return &grpcResponse, nil
}
func (server *Notificationserver) Watch(req *grpc_health_v1.HealthCheckRequest, srv grpc_health_v1.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "method Watch not implemented")
}

