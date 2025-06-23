package model

import (
	"github.com/lib/pq"
	"gorm.io/gorm"
)

type PolicyClass struct {
	gorm.Model
	Name string `gorm:"uniqueIndex"`
	// context params
	Obligations pq.StringArray `gorm:"type:text[]"`
	Conditions  pq.StringArray `gorm:"type:text[]"`
	Policies    []Policy       `gorm:"foreignKey:ClassName;references:Name;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (p *PolicyClass) Init(name string) {
	p.Name = name
}

type Policy struct {
	gorm.Model
	ClassName  string `gorm:"index"`
	Kind       string
	SubjectID  string         `gorm:"index"`
	Subject    *Entity        `gorm:"foreignKey:HashID;references:SubjectID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	ResourceID string         `gorm:"index"`
	Resource   *Entity        `gorm:"foreignKey:HashID;references:ResourceID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Actions    pq.StringArray `gorm:"type:text[]"`
}

const (
	POLICY_ALLOW PolicyKind = "ALLOW"
	POLICY_DENY  PolicyKind = "DENY"
)

type PolicyKind string

func (p PolicyKind) String() string {
	switch p {
	case POLICY_ALLOW:
		return "ALLOW"
	case POLICY_DENY:
		return "DENY"
	default:
		return ""
	}
}
