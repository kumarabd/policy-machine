package api

import (
	"fmt"

	"github.com/kumarabd/policy-machine/model"
)

type resourceBuild struct {
	Resource    *model.Resource
	Attributes  []*model.Attribute
	Assignments []*model.Assignment
}

func ResourceBuilderWithID(id string) *resourceBuild {
	// Create Subject
	res := &model.Resource{
		EntityID: id,
	}
	return &resourceBuild{
		Resource: res,
	}
}

func ResourceBuilder(name string, props map[string]string) *resourceBuild {
	// Create resource
	res := &model.Resource{}
	res.Init(name, props)

	// Add properties
	attributes := make([]*model.Attribute, 0)
	assignments := make([]*model.Assignment, 0)
	for k, v := range props {
		katt := &model.Attribute{}
		katt.Init(k, model.ResourceAttributeEntity, nil)
		vatt := &model.Attribute{}
		vatt.Init(v, model.ResourceAttributeEntity, map[string]string{
			k: v,
		})
		attributes = append(attributes, katt, vatt)

		kass := &model.Assignment{}
		kass.Init(vatt.DeepCopy(), katt.DeepCopy())
		resAss := &model.Assignment{}
		resAss.Init(res.DeepCopy(), vatt.DeepCopy())
		assignments = append(assignments, kass, resAss)
	}

	return &resourceBuild{
		Resource:    res,
		Attributes:  attributes,
		Assignments: assignments,
	}
}

func (res *resourceBuild) Fetch(h DataHandler) error {
	// Retrieve resource and its attributes
	err := h.FetchResource(res.Resource, true)
	if err != nil {
		return fmt.Errorf("unable to retrieve resource: %v", err)
	}

	return nil
}

func (res *resourceBuild) Create(h DataHandler) error {
	// Save resources and attributes first
	err := h.AddResource(res.Resource)
	if err != nil {
		return fmt.Errorf("unable to persist resource: %v", err)
	}

	err = h.AddAttributeBulk(res.Attributes)
	if err != nil {
		return fmt.Errorf("unable to persist attributes: %v", err)
	}

	// Save all assignments after both resources and attributes exist
	err = h.AddAssignmentBulk(res.Assignments)
	if err != nil {
		return fmt.Errorf("unable to persist assignments: %v", err)
	}
	return nil
}
