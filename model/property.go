package model

import (
	"github.com/kumarabd/policy-machine/utils"
	"gorm.io/gorm"
)

const (
	PropertyKey   PropertyString = "property_key"
	PropertyValue PropertyString = "property_value"
)

type PropertyString string

func (p PropertyString) String() string {
	return string(p)
}

type Property struct {
	gorm.Model
	ID    string `gorm:"primaryKey;uniqueIndex"`
	Key   string `gorm:"index"`
	Value string
}

func MapToProperty(in map[string]string) []*Property {
	props := make([]*Property, 0)
	for k, v := range in {
		// Generate id hash
		hash, _ := utils.GenerateJWTID(map[string]string{
			PropertyKey.String():   k,
			PropertyValue.String(): v,
		}, "secretKey")
		props = append(props, &Property{
			ID:    hash,
			Key:   k,
			Value: v,
		})
	}
	return props
}

func PropertyToMap(in []*Property) map[string]string {
	result := make(map[string]string, 0)
	for _, prop := range in {
		result[prop.Key] = prop.Value
	}
	return result
}
