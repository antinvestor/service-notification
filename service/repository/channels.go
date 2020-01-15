package repository

import (
	"antinvestor.com/service/notification/service/repository/models"
	"antinvestor.com/service/notification/utils"
	"context"
	"github.com/jinzhu/gorm"
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

func NewChannelRepository(ctx context.Context, env *utils.Env) ChannelRepository {
	return &channelRepository{readDb: env.GetRDb(ctx), writeDb: env.GeWtDb(ctx)}
}

func (repo *channelRepository) GetByID(id string) (*models.Channel, error) {
	channel := models.Channel{}
	err := repo.readDb.First(&channel, "channel_id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &channel, nil
}

func (repo *channelRepository) GetByMode(mode string) ([]models.Channel, error) {
	var channels []models.Channel

	err := repo.readDb.Find(&channels,
		"mode = ? OR ( mode = ?)", mode, models.ChannelModeTransceive).Error
	if err != nil {
		return nil, err
	}
	return channels, nil
}

func (repo *channelRepository) GetByModeAndTypeAndProductID(mode string, chType string, productId string) ([]models.Channel, error) {
	channels := []models.Channel{}
	err := repo.readDb.Find(&channels,
		"product_id = ? AND type = ? AND (mode = ? OR ( mode = ?))",
		productId, chType, mode, models.ChannelModeTransceive).Error
	if err != nil {
		return nil, err
	}
	return channels, nil
}

func (repo *channelRepository) Save(channel *models.Channel) error {
	return repo.writeDb.Save(channel).Error
}
