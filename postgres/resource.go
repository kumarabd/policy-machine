package postgres

import (
	"github.com/kumarabd/policy-machine/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (h *postgres) AddResource(r *model.Resource) error {
	return h.AddResourceBulk([]*model.Resource{r})
}

func (h *postgres) AddResourceBulk(resources []*model.Resource) error {
	processedEntities := make(map[string]struct{})
	return h.handler.Transaction(func(tx *gorm.DB) error {
		for _, r := range resources {
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

				// Handle Resource creation/update without FullSaveAssociations
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
					// Associate properties with resource
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
				processedEntities[r.EntityID] = struct{}{}
			}
		}
		return nil
	})
}

func (h *postgres) FetchResource(resource *model.Resource, preload bool) error {
	exec := h.handler.Where("entity_id = ?", resource.EntityID)
	if preload {
		exec = exec.Preload(clause.Associations)
	}

	result := exec.First(resource)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func (h *postgres) FetchResourceBulk(preload bool) ([]*model.Resource, error) {
	var resources []*model.Resource

	exec := h.handler
	if preload {
		exec = exec.Preload(clause.Associations)
	}

	result := exec.Find(&resources)
	if result.Error != nil {
		return nil, result.Error
	}
	return resources, nil
}
