package profile

import (
	"context"
	"google.golang.org/grpc"
)

func GetOrCreateProfileByContactDetail(ctx context.Context, conn *grpc.ClientConn, contact string)  (*ProfileObject, error) {

		profileService := NewProfileServiceClient(conn)

		contactRequest := ProfileContactRequest{
			Contact: contact,
		}
		return profileService.GetByContact(ctx, &contactRequest)
}


func GetProfileByID(ctx context.Context, conn *grpc.ClientConn, profileId string)  (*ProfileObject, error) {

		profileService := NewProfileServiceClient(conn)

		profileRequest := ProfileIDRequest{
			ID: profileId,
		}

		return profileService.GetByID(ctx, &profileRequest)
}

