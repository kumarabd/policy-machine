package postgres

import (
	"github.com/kumarabd/policy-machine/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (h *postgres) AddAssignment(r *model.Assignment) error {
	return h.AddAssignmentBulk([]*model.Assignment{r})
}

func (h *postgres) AddAssignmentBulk(Assignments []*model.Assignment) error {
	processedRelationships := make(map[string]struct{})
	return h.handler.Transaction(func(tx *gorm.DB) error {
		for _, r := range Assignments {
			if _, ok := processedRelationships[r.RelationshipID]; !ok {
				if err := tx.
					Session(&gorm.Session{FullSaveAssociations: true}).
					Where("relationship_id = ?", r.RelationshipID).
					FirstOrCreate(r).Error; err != nil {
					return err
				}
				processedRelationships[r.RelationshipID] = struct{}{}
			}
		}
		return nil
	})
}

func (h *postgres) FetchAssignment(ass *model.Assignment, preload bool) error {
	exec := h.handler.Where("relationship_id = ?", ass.RelationshipID)
	if preload {
		exec = exec.Preload(clause.Associations)
	}

	result := exec.First(ass)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func (h *postgres) FetchAssignmentBulk(preload bool) ([]*model.Assignment, error) {
	var assignments []*model.Assignment
	exec := h.handler
	if preload {
		exec = exec.Preload(clause.Associations)
	}

	result := exec.Find(&assignments)
	if result.Error != nil {
		return nil, result.Error
	}
	return assignments, nil
}
