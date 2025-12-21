package utility

import (
	"context"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	profilev1 "buf.build/gen/go/antinvestor/profile/protocolbuffers/go/profile/v1"
	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	KeyProfileName    = "au_name"
	KeyProfileImageID = "au_image"
)

// ExtractPropertyString extracts a string value from profile properties by key.
// Returns the value as a string (empty if not found or not a string).
func ExtractPropertyString(properties *structpb.Struct, key string) string {
	if properties == nil {
		return ""
	}
	props := properties.AsMap()
	if v, ok := props[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// PopulateContactLink resolves and populates a ContactLink with profile information
// based on the specified contact type. It first checks if Detail is directly available,
// then tries to fetch by ContactId, and finally by ProfileId.
// Updates the contact in place with ProfileId, ProfileType, ContactId, and Detail.
func PopulateContactLink(
	ctx context.Context,
	profileCli profilev1connect.ProfileServiceClient,
	contact *commonv1.ContactLink,
	contactType profilev1.ContactType,
) (*commonv1.ContactLink, error) {

	if contact.GetDetail() != "" {
		return contact, nil
	}

	if contact.GetContactId() != "" {
		result, err := profileCli.GetByContact(ctx, connect.NewRequest(&profilev1.GetByContactRequest{Contact: contact.GetContactId()}))
		if err != nil {
			return nil, err
		}

		profile := result.Msg.GetData()

		for _, c := range profile.GetContacts() {
			if c.GetType() == contactType {
				if c.GetId() == contact.GetContactId() {
					contact.ProfileType = profile.GetType().String()
					contact.ProfileId = profile.GetId()
					contact.ProfileName = ExtractPropertyString(profile.GetProperties(), KeyProfileName)
					contact.ProfileImageId = ExtractPropertyString(profile.GetProperties(), KeyProfileImageID)
					contact.Detail = c.GetDetail()
					return contact, nil
				}
			}
		}
	}

	if contact.GetProfileId() != "" {
		result, err := profileCli.GetById(ctx, connect.NewRequest(&profilev1.GetByIdRequest{Id: contact.GetProfileId()}))
		if err != nil {
			return nil, err
		}

		profile := result.Msg.GetData()

		for _, c := range profile.GetContacts() {
			if c.GetType() == contactType {
				contact.ProfileType = profile.GetType().String()
				contact.ContactId = c.GetId()
				contact.ProfileName = ExtractPropertyString(profile.GetProperties(), KeyProfileName)
				contact.ProfileImageId = ExtractPropertyString(profile.GetProperties(), KeyProfileImageID)
				contact.Detail = c.GetDetail()
				return contact, nil
			}
		}
	}

	return contact, nil
}
