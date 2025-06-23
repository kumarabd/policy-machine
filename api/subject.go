package api

import (
	"fmt"

	"github.com/kumarabd/policy-machine/model"
)

type subjectBuild struct {
	Subject     *model.Subject
	Attributes  []*model.Attribute
	Assignments []*model.Assignment
}

func SubjectBuilderWithID(id string) *subjectBuild {
	// Create Subject
	sub := &model.Subject{
		EntityID: id,
	}
	return &subjectBuild{
		Subject: sub,
	}
}

func SubjectBuilder(name string, props map[string]string) *subjectBuild {
	// Create Subject
	sub := &model.Subject{}
	sub.Init(name, props)

	// Add properties
	attributes := make([]*model.Attribute, 0)
	assignments := make([]*model.Assignment, 0)
	for k, v := range props {
		katt := &model.Attribute{}
		katt.Init(k, model.SubjectAttributeEntity, nil)
		vatt := &model.Attribute{}
		vatt.Init(v, model.SubjectAttributeEntity, map[string]string{
			k: v,
		})
		attributes = append(attributes, katt, vatt)

		kass := &model.Assignment{}
		kass.Init(vatt.DeepCopy(), katt.DeepCopy())
		subAss := &model.Assignment{}
		subAss.Init(sub.DeepCopy(), vatt.DeepCopy())
		assignments = append(assignments, kass, subAss)
	}

	return &subjectBuild{
		Subject:     sub,
		Attributes:  attributes,
		Assignments: assignments,
	}
}

func (sub *subjectBuild) Fetch(h DataHandler) error {
	// Retrieve subject and its attributes
	err := h.FetchSubject(sub.Subject, true)
	if err != nil {
		return fmt.Errorf("unable to retrieve subject: %v", err)
	}

	return nil
}

func (sub *subjectBuild) Create(h DataHandler) error {
	// Save Subjects and attributes first
	err := h.AddSubjectIfDoesntExist(sub.Subject)
	if err != nil {
		return fmt.Errorf("unable to persist subject: %v", err)
	}

	err = h.AddAttributeBulk(sub.Attributes)
	if err != nil {
		return fmt.Errorf("unable to persist attributes: %v", err)
	}

	// Save all assignments after both Subjects and attributes exist
	err = h.AddAssignmentBulk(sub.Assignments)
	if err != nil {
		return fmt.Errorf("unable to persist assignments: %v", err)
	}
	return nil
}
