package postgres

import (
	"github.com/kumarabd/policy-machine/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (h *postgres) AddPolicyClass(r *model.PolicyClass) error {
	return h.AddPolicyClassBulk([]*model.PolicyClass{r})
}

func (h *postgres) AddPolicyClassBulk(resources []*model.PolicyClass) error {
	processedEntities := make(map[string]struct{})
	return h.handler.Transaction(func(tx *gorm.DB) error {
		for _, r := range resources {
			if _, ok := processedEntities[r.Name]; !ok {
				if err := tx.
					Session(&gorm.Session{FullSaveAssociations: true}).
					Where("name = ?", r.Name).
					FirstOrCreate(r).Error; err != nil {
					return err
				}
				processedEntities[r.Name] = struct{}{}
			}
		}
		return nil
	})
}

func (h *postgres) FetchPolicyClass(policyClass *model.PolicyClass, preload bool) error {
	exec := h.handler.Where("name = ?", policyClass.Name)
	if preload {
		exec = exec.Preload(clause.Associations)
	}

	result := exec.First(policyClass)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func (h *postgres) FetchPolicyClassBulk(preload bool) ([]*model.PolicyClass, error) {
	var resources []*model.PolicyClass
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
