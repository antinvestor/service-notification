package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"buf.build/gen/go/antinvestor/profile/connectrpc/go/v2/profile/v1/profilev1connect"
	profilev1 "buf.build/gen/go/antinvestor/profile/protocolbuffers/go/profile/v1"
	"buf.build/gen/go/antinvestor/tenancy/connectrpc/go/v2/tenancy/v1/tenancyv1connect"
	tenancyv1 "buf.build/gen/go/antinvestor/tenancy/protocolbuffers/go/tenancy/v1"
	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
)

// ProfileStub is an in-process profile service: every contact id resolves to a profile
// owning that single contact, typed by its shape (email address or phone number).
// Contact ids containing "missing" are not found, so tests can exercise that path.
type ProfileStub struct {
	profilev1connect.UnimplementedProfileServiceHandler
}

func profileFor(profileID, contactID string) *profilev1.ProfileObject {
	contactType := profilev1.ContactType_MSISDN
	detail := "+254700000000"
	if strings.Contains(contactID, "@") || strings.Contains(contactID, "email") {
		contactType = profilev1.ContactType_EMAIL
		detail = contactID + "@example.com"
	}
	return &profilev1.ProfileObject{
		Id:   profileID,
		Type: profilev1.ProfileType_PERSON,
		Contacts: []*profilev1.ContactObject{{
			Id: contactID, Type: contactType, Detail: detail, Verified: true,
		}},
	}
}

func (ProfileStub) GetByContact(_ context.Context, req *profilev1.GetByContactRequest) (*profilev1.GetByContactResponse, error) {
	contact := req.GetContact()
	if contact == "" || strings.Contains(contact, "missing") {
		return nil, connect.NewError(connect.CodeNotFound, "profile not found")
	}
	return &profilev1.GetByContactResponse{Data: profileFor("profile-of-"+contact, contact)}, nil
}

func (ProfileStub) GetById(_ context.Context, req *profilev1.GetByIdRequest) (*profilev1.GetByIdResponse, error) {
	id := req.GetId()
	if id == "" || strings.Contains(id, "missing") {
		return nil, connect.NewError(connect.CodeNotFound, "profile not found")
	}
	return &profilev1.GetByIdResponse{Data: profileFor(id, "contact-of-"+id)}, nil
}

// TenancyStub answers partition lookups with an empty partition.
type TenancyStub struct {
	tenancyv1connect.UnimplementedTenancyServiceHandler
}

func (TenancyStub) GetPartition(_ context.Context, req *tenancyv1.GetPartitionRequest) (*tenancyv1.GetPartitionResponse, error) {
	return &tenancyv1.GetPartitionResponse{Data: &tenancyv1.PartitionObject{Id: req.GetId(), Name: "test"}}, nil
}

// NewPeerClients serves both stubs over HTTP for the lifetime of the test.
func NewPeerClients(t *testing.T) (profilev1connect.ProfileServiceClient, tenancyv1connect.TenancyServiceClient) {
	t.Helper()
	mux := http.NewServeMux()
	server := connect.NewServer()
	profilev1connect.RegisterProfileServiceHandler(server, ProfileStub{})
	tenancyv1connect.RegisterTenancyServiceHandler(server, TenancyStub{})
	connecthttp.Mount(mux, server)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return profilev1connect.NewProfileServiceClient(connect.NewClient(connecthttp.NewTransport(srv.Client(), srv.URL))),
		tenancyv1connect.NewTenancyServiceClient(connect.NewClient(connecthttp.NewTransport(srv.Client(), srv.URL)))
}
