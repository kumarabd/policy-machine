package api

import (
	"fmt"

	"github.com/kumarabd/policy-machine/model"
	"github.com/kumarabd/policy-machine/postgres"
	"github.com/lib/pq"
)

type policyClassBuild struct {
	name string
}

type policyBuild struct {
	Class        string
	Policies     []*model.Policy
	Associations []*model.Association
	Prohibitions []*model.Prohibition
}

func PolicyClassBuilder(className string) *policyClassBuild {
	return &policyClassBuild{
		name: className,
	}
}

func (res *policyClassBuild) Exec(h DataHandler) error {
	node := &model.PolicyClass{}
	node.Init(res.name)
	return h.AddPolicyClass(node)
}

func DirectPolicyBuilder(className string, kind model.PolicyKind, subject map[string]string, resource map[string]string, actions []string) (*policyBuild, error) {
	var sourceNode, targetNode model.Entity

	if name, ok := subject[model.NameAttribute.String()]; ok {
		sub := &model.Subject{}
		sub.Init(name, subject)
		sourceNode = sub.DeepCopy()
	} else {
		for k, v := range subject {
			name = v
			if v == "*" {
				delete(subject, k)
				name = k
			}
			sub := &model.Attribute{}
			sub.Init(name, model.SubjectAttributeEntity, subject)
			sourceNode = sub.DeepCopy()
			break
		}
	}

	if name, ok := resource[model.NameAttribute.String()]; ok {
		sub := &model.Resource{}
		sub.Init(name, resource)
		targetNode = sub.DeepCopy()
	} else {
		for k, v := range resource {
			name = v
			if v == "*" {
				delete(resource, k)
				name = k
			}
			sub := &model.Attribute{}
			sub.Init(name, model.ResourceAttributeEntity, resource)
			targetNode = sub.DeepCopy()
			break
		}
	}

	policy := &model.Policy{
		ClassName:  className,
		Kind:       kind.String(),
		SubjectID:  sourceNode.GetID(),
		ResourceID: targetNode.GetID(),
		Subject:    &sourceNode,
		Resource:   &targetNode,
		Actions:    pq.StringArray(actions),
	}

	return &policyBuild{
		Class:        className,
		Policies:     []*model.Policy{policy},
		Associations: make([]*model.Association, 0),
		Prohibitions: make([]*model.Prohibition, 0),
	}, nil
}

func (res *policyBuild) Exec(h DataHandler) error {
	for _, policy := range res.Policies {
		if policy == nil {
			continue
		}
		// create association mappings
		if err := h.FetchEntityForID(policy.SubjectID, policy.Subject); err != nil {
			return fmt.Errorf("failed to fetch subject for policy: %v", err)
		}
		if err := h.FetchEntityForID(policy.ResourceID, policy.Resource); err != nil {
			return fmt.Errorf("failed to fetch resource for policy: %v", err)
		}

		switch policy.Kind {
		case model.POLICY_ALLOW.String():
			ass := &model.Association{}
			ass.Init(*policy.Subject, *policy.Resource, policy.Actions, res.Class)
			err := h.FetchAssociation(ass, true)
			if err != nil {
				if err != postgres.ErrNotFound {
					return fmt.Errorf("unable to fetch policy association: %v", err)
				}
				// Association doesn't exist, add to batch for bulk creation
				res.Associations = append(res.Associations, ass)
			} else {
				// Association exists, just add actions
				if er := h.AddActionsToAssociations(ass, policy.Actions); er != nil {
					return fmt.Errorf("failed to add actions to association: %v", er)
				}
			}
		case model.POLICY_DENY.String():
			proh := &model.Prohibition{}
			proh.Init(*policy.Subject, *policy.Resource, policy.Actions, res.Class)
			err := h.FetchProhibition(proh, true)
			if err != nil {
				if err != postgres.ErrNotFound {
					return fmt.Errorf("unable to fetch prohibition: %v", err)
				}
				// Prohibition doesn't exist, add to batch for bulk creation
				res.Prohibitions = append(res.Prohibitions, proh)
			} else {
				// Prohibition exists, just add actions
				if er := h.AddActionsToProhibitions(proh, policy.Actions); er != nil {
					return fmt.Errorf("failed to add actions to prohibition: %v", er)
				}
			}
		}
	}

	// Only bulk create new associations
	if len(res.Associations) > 0 {
		err := h.AddAssociationBulk(res.Associations)
		if err != nil {
			return fmt.Errorf("unable to persist associations: %v", err)
		}
	}

	// Only bulk create new prohibitions
	if len(res.Prohibitions) > 0 {
		err := h.AddProhibitionBulk(res.Prohibitions)
		if err != nil {
			return fmt.Errorf("unable to persist prohibitions: %v", err)
		}
	}

	// Persist the policies
	err := h.AddPolicyBulk(res.Policies)
	if err != nil {
		return fmt.Errorf("unable to persist policies: %v", err)
	}

	return nil
}
