package business

import (
	"antinvestor.com/service/notification/grpc/profile"
	"antinvestor.com/service/notification/utils"
	"context"
)

func getOrCreateProfileByContact(env *utils.Env, ctx context.Context, contact string)  (*profile.ProfileObject, error) {

		profileService := profile.NewProfileServiceClient(env.GetProfileServiceConn())

		contactRequest := profile.ProfileContactRequest{
			Contact: contact,
		}
		return profileService.GetByContact(ctx, &contactRequest)
}

