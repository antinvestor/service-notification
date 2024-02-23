package repository

import (
	"context"

	"github.com/antinvestor/service-notification/service/models"
	"github.com/pitabwire/frame"
	"gorm.io/gorm"
)

type RouteRepository interface {
	GetByID(id string) (*models.Route, error)
	GetByModeTypeAndPartitionID(mode string, routeType string, partitionId string) ([]*models.Route, error)
	GetByMode(mode string) ([]*models.Route, error)
	Save(channel *models.Route) error
}

type routeRepository struct {
	readDb  *gorm.DB
	writeDb *gorm.DB
}

func NewRouteRepository(ctx context.Context, service *frame.Service) RouteRepository {
	return &routeRepository{readDb: service.DB(ctx, true), writeDb: service.DB(ctx, false)}
}

func (repo *routeRepository) GetByID(id string) (*models.Route, error) {
	route := models.Route{}
	err := repo.readDb.First(&route, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &route, nil
}

func (repo *routeRepository) GetByMode(mode string) ([]*models.Route, error) {
	var routes []*models.Route

	err := repo.readDb.Find(&routes,
		"mode = ? OR ( mode = ?)", mode, models.RouteModeTransceive).Error
	if err != nil {
		return nil, err
	}
	return routes, nil
}

func (repo *routeRepository) GetByModeTypeAndPartitionID(mode string, routeType string, partitionId string) ([]*models.Route, error) {
	var routes []*models.Route

	err := repo.readDb.Find(&routes,
		"partition_id = ? AND ( route_type = ? OR route_type = ? ) AND (mode = ? OR ( mode = ?))",
		partitionId, "any", routeType, mode, models.RouteModeTransceive).Error
	if err != nil {
		return nil, err
	}
	return routes, nil
}

func (repo *routeRepository) Save(route *models.Route) error {
	return repo.writeDb.Save(route).Error
}
