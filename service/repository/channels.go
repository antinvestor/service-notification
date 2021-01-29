package repository

import (
	"context"
	"github.com/antinvestor/service-notification/service/repository/models"
	"github.com/go-errors/errors"
	"github.com/pitabwire/frame"
	"gorm.io/gorm"
)

type ChannelRepository interface {
	GetByID(id string) (*models.Channel, error)
	GetByModeAndTypeAndProductID(mode string, chanType string, productId string) ([]models.Channel, error)
	GetByMode(mode string) ([]models.Channel, error)
	Save(channel *models.Channel) error
}

type channelRepository struct {
	readDb  *gorm.DB
	writeDb *gorm.DB
}

func NewChannelRepository(ctx context.Context, service *frame.Service) ChannelRepository {
	return &channelRepository{readDb: service.DB(ctx,true), writeDb: service.DB(ctx,false)}
}

func (repo *channelRepository) GetByID(id string) (*models.Channel, error) {
	channel := models.Channel{}
	err := repo.readDb.First(&channel, "id = ?", id).Error
	if err != nil {
		return nil, errors.Wrap(err, 1)
	}
	return &channel, nil
}

func (repo *channelRepository) GetByMode(mode string) ([]models.Channel, error) {
	var channels []models.Channel

	err := repo.readDb.Find(&channels,
		"mode = ? OR ( mode = ?)", mode, models.ChannelModeTransceive).Error
	if err != nil {
		return nil, errors.Wrap(err, 1)
	}
	return channels, nil
}

func (repo *channelRepository) GetByModeAndTypeAndProductID(mode string, chType string, productId string) ([]models.Channel, error) {
	channels := []models.Channel{}
	err := repo.readDb.Find(&channels,
		"product_id = ? AND type = ? AND (mode = ? OR ( mode = ?))",
		productId, chType, mode, models.ChannelModeTransceive).Error
	if err != nil {
		return nil, errors.Wrap(err, 1)
	}
	return channels, nil
}

func (repo *channelRepository) Save(channel *models.Channel) error {
	return repo.writeDb.Save(channel).Error
}
