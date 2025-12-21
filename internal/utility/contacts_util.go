package utility

import (
	"context"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	profilev1 "buf.build/gen/go/antinvestor/profile/protocolbuffers/go/profile/v1"
	"connectrpc.com/connect"
	"github.com/pitabwire/util"
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
	log := util.Log(ctx)

	if contact.GetDetail() != "" {
		log.WithField("detail", contact.GetDetail()).Debug("PopulateContactLink: Detail already set, returning as-is")
		return contact, nil
	}

	if contact.GetContactId() != "" {
		log.WithFields(map[string]any{
			"contact_id":   contact.GetContactId(),
			"contact_type": contactType.String(),
		}).Debug("PopulateContactLink: Looking up by ContactId")

		result, err := profileCli.GetByContact(ctx, connect.NewRequest(&profilev1.GetByContactRequest{Contact: contact.GetContactId()}))
		if err != nil {
			log.WithError(err).WithField("contact_id", contact.GetContactId()).Error("PopulateContactLink: GetByContact failed")
			return nil, err
		}

		profile := result.Msg.GetData()
		log.WithFields(map[string]any{
			"profile_id":     profile.GetId(),
			"profile_type":   profile.GetType().String(),
			"contacts_count": len(profile.GetContacts()),
		}).Debug("PopulateContactLink: GetByContact returned profile")

		for i, c := range profile.GetContacts() {
			log.WithFields(map[string]any{
				"index":            i,
				"contact_id":       c.GetId(),
				"contact_detail":   c.GetDetail(),
				"contact_type":     c.GetType().String(),
				"looking_for_id":   contact.GetContactId(),
				"looking_for_type": contactType.String(),
			}).Debug("PopulateContactLink: Checking contact from profile")

			if c.GetType() == contactType {
				if c.GetId() == contact.GetContactId() {
					log.WithFields(map[string]any{
						"matched_id":     c.GetId(),
						"matched_detail": c.GetDetail(),
					}).Debug("PopulateContactLink: Found matching contact by ID and type")
					contact.ProfileType = profile.GetType().String()
					contact.ProfileId = profile.GetId()
					contact.ProfileName = ExtractPropertyString(profile.GetProperties(), KeyProfileName)
					contact.ProfileImageId = ExtractPropertyString(profile.GetProperties(), KeyProfileImageID)
					contact.Detail = c.GetDetail()
					return contact, nil
				}
			}
		}
		log.WithFields(map[string]any{
			"contact_id":   contact.GetContactId(),
			"contact_type": contactType.String(),
		}).Warn("PopulateContactLink: No matching contact found by ContactId lookup")
	}

	if contact.GetProfileId() != "" {
		log.WithField("profile_id", contact.GetProfileId()).Debug("PopulateContactLink: Looking up by ProfileId")

		result, err := profileCli.GetById(ctx, connect.NewRequest(&profilev1.GetByIdRequest{Id: contact.GetProfileId()}))
		if err != nil {
			log.WithError(err).WithField("profile_id", contact.GetProfileId()).Error("PopulateContactLink: GetById failed")
			return nil, err
		}

		profile := result.Msg.GetData()
		log.WithFields(map[string]any{
			"profile_id":     profile.GetId(),
			"contacts_count": len(profile.GetContacts()),
		}).Debug("PopulateContactLink: GetById returned profile")

		for i, c := range profile.GetContacts() {
			log.WithFields(map[string]any{
				"index":            i,
				"contact_id":       c.GetId(),
				"contact_detail":   c.GetDetail(),
				"contact_type":     c.GetType().String(),
				"looking_for_type": contactType.String(),
			}).Debug("PopulateContactLink: Checking contact from profile (by ProfileId)")

			if c.GetType() == contactType {
				log.WithFields(map[string]any{
					"matched_id":     c.GetId(),
					"matched_detail": c.GetDetail(),
				}).Debug("PopulateContactLink: Found matching contact by type")
				contact.ProfileType = profile.GetType().String()
				contact.ContactId = c.GetId()
				contact.ProfileName = ExtractPropertyString(profile.GetProperties(), KeyProfileName)
				contact.ProfileImageId = ExtractPropertyString(profile.GetProperties(), KeyProfileImageID)
				contact.Detail = c.GetDetail()
				return contact, nil
			}
		}
		log.WithFields(map[string]any{
			"profile_id":   contact.GetProfileId(),
			"contact_type": contactType.String(),
		}).Warn("PopulateContactLink: No matching contact found by ProfileId lookup")
	}

	log.WithFields(map[string]any{
		"contact_id":   contact.GetContactId(),
		"profile_id":   contact.GetProfileId(),
		"detail":       contact.GetDetail(),
		"contact_type": contactType.String(),
	}).Warn("PopulateContactLink: Returning contact without resolving Detail")
	return contact, nil
}
