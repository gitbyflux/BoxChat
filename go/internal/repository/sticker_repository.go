package repository

import (
	"boxchat/internal/models"

	"gorm.io/gorm"
)

// StickerRepositoryImpl implements StickerRepository interface
type StickerRepositoryImpl struct {
	db *gorm.DB
}

// NewStickerRepository creates a new StickerRepositoryImpl
func NewStickerRepository(db *gorm.DB) *StickerRepositoryImpl {
	return &StickerRepositoryImpl{db: db}
}

// CreatePack creates a new sticker pack in the database
func (r *StickerRepositoryImpl) CreatePack(pack *models.StickerPack) error {
	return r.db.Create(pack).Error
}

// GetPackByID retrieves a sticker pack by ID
func (r *StickerRepositoryImpl) GetPackByID(id uint) (*models.StickerPack, error) {
	var pack models.StickerPack
	err := r.db.Preload("Owner").Preload("Stickers").First(&pack, id).Error
	if err != nil {
		return nil, err
	}
	return &pack, nil
}

// GetAllPacks retrieves all sticker packs from the database
func (r *StickerRepositoryImpl) GetAllPacks() ([]*models.StickerPack, error) {
	var packs []*models.StickerPack
	err := r.db.Find(&packs).Error
	return packs, err
}

// UpdatePack updates an existing sticker pack in the database
func (r *StickerRepositoryImpl) UpdatePack(pack *models.StickerPack) error {
	return r.db.Save(pack).Error
}

// DeletePack deletes a sticker pack by ID
func (r *StickerRepositoryImpl) DeletePack(id uint) error {
	return r.db.Delete(&models.StickerPack{}, id).Error
}

// CreateSticker creates a new sticker in the database
func (r *StickerRepositoryImpl) CreateSticker(sticker *models.Sticker) error {
	return r.db.Create(sticker).Error
}

// GetStickerByID retrieves a sticker by ID
func (r *StickerRepositoryImpl) GetStickerByID(id uint) (*models.Sticker, error) {
	var sticker models.Sticker
	err := r.db.First(&sticker, id).Error
	if err != nil {
		return nil, err
	}
	return &sticker, nil
}

// DeleteSticker deletes a sticker by ID
func (r *StickerRepositoryImpl) DeleteSticker(id uint) error {
	return r.db.Delete(&models.Sticker{}, id).Error
}

// GetPacksByOwner retrieves all sticker packs owned by a user
func (r *StickerRepositoryImpl) GetPacksByOwner(ownerID uint) ([]*models.StickerPack, error) {
	var packs []*models.StickerPack
	err := r.db.Where("owner_id = ?", ownerID).Find(&packs).Error
	return packs, err
}

// GetStickersByPack retrieves all stickers from a pack
func (r *StickerRepositoryImpl) GetStickersByPack(packID uint) ([]*models.Sticker, error) {
	var stickers []*models.Sticker
	err := r.db.Where("pack_id = ?", packID).Find(&stickers).Error
	return stickers, err
}
