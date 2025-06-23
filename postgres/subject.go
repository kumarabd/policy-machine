package postgres

import (
	"github.com/kumarabd/policy-machine/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (h *postgres) AddSubjectIfDoesntExist(r *model.Subject) error {
	return h.handler.Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Session(&gorm.Session{FullSaveAssociations: true}).
			Where("entity_id = ?", r.EntityID).
			Create(r).Error; err != nil {
			return err
		}
		return nil
	})
}

func (h *postgres) AddSubject(r *model.Subject) error {
	return h.AddSubjectBulk([]*model.Subject{r})
}

func (h *postgres) AddSubjectBulk(subjects []*model.Subject) error {
	processedEntities := make(map[string]struct{})
	return h.handler.Transaction(func(tx *gorm.DB) error {
		for _, r := range subjects {
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

				// Handle Subject creation/update without FullSaveAssociations
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
					// Associate properties with subject
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

func (h *postgres) FetchOrCreateSubject(subject *model.Subject, preload bool) error {
	err := h.FetchSubject(subject, preload)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return h.AddSubject(subject)
		}
		return err
	}
	return nil
}

func (h *postgres) FetchSubject(subject *model.Subject, preload bool) error {
	exec := h.handler.Where("entity_id = ?", subject.EntityID)
	if preload {
		exec = exec.Preload(clause.Associations)
	}

	result := exec.First(subject)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func (h *postgres) FetchSubjectBulk(preload bool) ([]*model.Subject, error) {
	var subjects []*model.Subject

	exec := h.handler
	if preload {
		exec = exec.Preload(clause.Associations)
	}

	result := exec.Find(&subjects)
	if result.Error != nil {
		return nil, result.Error
	}
	return subjects, nil
}
