package model

import (
	"github.com/lib/pq"
	"gorm.io/gorm"
)

const (
	ProhibitionRelationship RelationshipString = "prohibition"
)

// Prohibition represents negative permissions in NGAC
// A prohibition denies specific operations between subjects and object attributes
type Prohibition struct {
	gorm.Model
	RelationshipID string         `gorm:"primaryKey;uniqueIndex"`
	Relationship   *Relationship  `gorm:"foreignKey:HashID;references:RelationshipID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Verbs          pq.StringArray `gorm:"type:text[];not null"` // Operations denied
	ClassName      string         `gorm:"index;not null"`       // Policy class
}

func (p *Prohibition) DeepCopy() Relationship {
	relationship := *p.Relationship
	return relationship
}

// AddVerbs adds denied operations to this prohibition
func (p *Prohibition) AddVerbs(verbs []string) {
	temp := make(map[string]struct{})
	for _, verb := range p.Verbs {
		temp[verb] = struct{}{}
	}
	for _, verb := range verbs {
		if _, exists := temp[verb]; !exists {
			p.Verbs = append(p.Verbs, verb)
		}
	}
}

// Init initializes a prohibition with NGAC-compliant structure
func (p *Prohibition) Init(from Entity, to Entity, verbs []string, className string) {
	p.Relationship = &Relationship{}
	p.Relationship.Init(from, to, ProhibitionRelationship)
	p.RelationshipID = p.Relationship.HashID
	p.Verbs = verbs
	p.ClassName = className
}

// HasOperation checks if this prohibition denies the specified action
func (p *Prohibition) HasOperation(action string) bool {
	for _, op := range p.Verbs {
		if op == action || op == "*" {
			return true
		}
	}
	return false
}
