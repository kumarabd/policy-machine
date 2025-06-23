package postgres

import (
	"github.com/kumarabd/policy-machine/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (h *postgres) AddAttribute(r *model.Attribute) error {
	return h.AddAttributeBulk([]*model.Attribute{r})
}

func (h *postgres) AddAttributeBulk(attributes []*model.Attribute) error {
	processedEntities := make(map[string]struct{})
	return h.handler.Transaction(func(tx *gorm.DB) error {
		for _, r := range attributes {
			if _, ok := processedEntities[r.EntityID]; !ok {
				// Handle Entity creation/update separately
				if r.Entity != nil {
					if err := tx.
						Clauses(clause.OnConflict{
							Columns:   []clause.Column{{Name: "hash_id"}},
							UpdateAll: true,
						}).
						Create(r.Entity).Error; err != nil {
						return err
					}
				}

				// Handle Attribute creation/update without FullSaveAssociations
				if err := tx.
					Clauses(clause.OnConflict{
						Columns:   []clause.Column{{Name: "entity_id"}},
						UpdateAll: true,
					}).
					Create(r).Error; err != nil {
					return err
				}

				// Handle Properties if they exist
				if len(r.Properties) > 0 {
					for _, prop := range r.Properties {
						if err := tx.
							Clauses(clause.OnConflict{
								DoNothing: true,
							}).
							Create(prop).Error; err != nil {
							return err
						}
					}
					// Associate properties with attribute
					if err := tx.Model(r).Association("Properties").Replace(r.Properties); err != nil {
						return err
					}
				}

				// Handle Assignments if they exist
				if len(r.Assignments) > 0 {
					for _, assignment := range r.Assignments {
						if err := tx.
							Clauses(clause.OnConflict{
								Columns:   []clause.Column{{Name: "hash_id"}},
								UpdateAll: true,
							}).
							Create(&assignment).Error; err != nil {
							return err
						}
					}
				}

				// Handle Associations if they exist
				if len(r.Associations) > 0 {
					for _, association := range r.Associations {
						if err := tx.
							Clauses(clause.OnConflict{
								Columns:   []clause.Column{{Name: "hash_id"}},
								UpdateAll: true,
							}).
							Create(&association).Error; err != nil {
							return err
						}
					}
				}
				processedEntities[r.EntityID] = struct{}{}
			}
		}
		return nil
	})
}

func (h *postgres) FetchOrCreateAttribute(att *model.Attribute, preload bool) error {
	err := h.FetchAttribute(att, preload)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return h.AddAttribute(att)
		}
		return err
	}
	return nil
}

func (h *postgres) FetchAttribute(att *model.Attribute, preload bool) error {
	exec := h.handler.Where("entity_id = ?", att.EntityID)
	if preload {
		exec = exec.Preload(clause.Associations)
	}

	result := exec.First(att)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func (h *postgres) FetchAttributeBulk(preload bool) ([]*model.Attribute, error) {
	var attributes []*model.Attribute

	exec := h.handler
	if preload {
		exec = exec.Preload(clause.Associations)
	}

	result := exec.Find(&attributes)
	if result.Error != nil {
		return nil, result.Error
	}
	return attributes, nil
}
