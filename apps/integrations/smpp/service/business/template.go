package business

import (
	"context"

	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	"buf.build/gen/go/antinvestor/tenancy/connectrpc/go/tenancy/v1/tenancyv1connect"
	"github.com/antinvestor/service-notification/apps/integrations/smpp/service/events"
	"github.com/antinvestor/service-notification/apps/integrations/smpp/service/models"
	"github.com/antinvestor/service-notification/apps/integrations/smpp/service/repository"
	"github.com/pitabwire/frame/datastore/pool"
	fevents "github.com/pitabwire/frame/events"
	"github.com/pitabwire/frame/security"
)

type TemplateBusiness interface {
	Get(ctx context.Context, id string) (*models.Template, error)
	Store(ctx context.Context, name string) (*models.Template, error)
}

func NewTemplateBusiness(ctx context.Context, dbPool pool.Pool, eventsMan fevents.Manager,
	profileCli profilev1connect.ProfileServiceClient,
	tenancyCli tenancyv1connect.TenancyServiceClient) (TemplateBusiness, error) {
	if dbPool == nil || profileCli == nil || tenancyCli == nil {
		return nil, ErrorInitializationFail
	}

	return &templateBusiness{
		dbPool:     dbPool,
		eventsMan:  eventsMan,
		profileCli: profileCli,
	}, nil
}

type templateBusiness struct {
	dbPool     pool.Pool
	eventsMan  fevents.Manager
	profileCli profilev1connect.ProfileServiceClient
}

func (nb *templateBusiness) Get(ctx context.Context, id string) (*models.Template, error) {
	templateRepository := repository.NewTemplateRepository(ctx, nb.dbPool)
	template, err := templateRepository.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return template, nil
}

func (nb *templateBusiness) Store(ctx context.Context, name string) (*models.Template, error) {
	template := models.Template{
		Name: name,
	}

	authClaims := security.ClaimsFromContext(ctx)
	if authClaims != nil {
		template.TenantID = authClaims.TenantID
		template.PartitionID = authClaims.PartitionID
		template.AccessID = authClaims.AccessID
	}

	template.GenID(ctx)
	// Queue in message for further processing
	err := nb.eventsMan.Emit(ctx, events.TemplateSaveEvent, template)
	if err != nil {
		return nil, err
	}

	return &template, nil
}
