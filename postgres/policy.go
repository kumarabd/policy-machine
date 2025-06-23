package postgres

import (
	"github.com/kumarabd/policy-machine/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (h *postgres) AddPolicy(r *model.Policy) error {
	return h.AddPolicyBulk([]*model.Policy{r})
}

func (h *postgres) AddPolicyBulk(resources []*model.Policy) error {
	processedPolicies := make(map[uint]struct{})
	return h.handler.Transaction(func(tx *gorm.DB) error {
		for _, r := range resources {
			if _, ok := processedPolicies[r.ID]; !ok {
				if err := tx.
					Where("id = ?", r.ID).
					FirstOrCreate(r).Error; err != nil {
					return err
				}
				processedPolicies[r.ID] = struct{}{}
			}
		}
		return nil
	})
}

func (h *postgres) FetchPolicy(policy *model.Policy, preload bool) error {
	exec := h.handler.Where("id = ?", policy.ID)
	if preload {
		exec = exec.Preload(clause.Associations)
	}

	result := exec.First(policy)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func (h *postgres) FetchPolicyBulk(preload bool) ([]*model.Policy, error) {
	var policies []*model.Policy
	exec := h.handler
	if preload {
		exec = exec.Preload(clause.Associations)
	}

	result := exec.Find(&policies)
	if result.Error != nil {
		return nil, result.Error
	}
	return policies, nil
}
