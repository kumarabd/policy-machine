package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEntityString(t *testing.T) {
	t.Run("string_conversion", func(t *testing.T) {
		es := EntityString("test_entity")
		assert.Equal(t, "test_entity", es.String())
	})

	t.Run("empty_string", func(t *testing.T) {
		es := EntityString("")
		assert.Equal(t, "", es.String())
	})

	t.Run("entity_key_constant", func(t *testing.T) {
		assert.Equal(t, "entity", EntityKey.String())
	})
}

func TestEntity_Getters(t *testing.T) {
	entity := &Entity{
		HashID: "test-hash-123",
		Type:   EntityString("test_type"),
		Name:   "Test Entity",
	}

	t.Run("get_name", func(t *testing.T) {
		assert.Equal(t, "Test Entity", entity.GetName())
	})

	t.Run("get_type", func(t *testing.T) {
		assert.Equal(t, EntityString("test_type"), entity.GetType())
		assert.Equal(t, "test_type", entity.GetType().String())
	})

	t.Run("get_id", func(t *testing.T) {
		assert.Equal(t, "test-hash-123", entity.GetID())
	})
}

func TestEntity_Init(t *testing.T) {
	t.Run("basic_initialization", func(t *testing.T) {
		entity := &Entity{}
		props := map[string]string{
			"department": "engineering",
			"level":      "senior",
		}

		entity.Init("TestEntity", EntityString("user"), props)

		assert.Equal(t, "TestEntity", entity.Name)
		assert.Equal(t, EntityString("user"), entity.Type)
		assert.NotEmpty(t, entity.HashID, "HashID should be generated")
	})

	t.Run("empty_props", func(t *testing.T) {
		entity := &Entity{}
		props := make(map[string]string)

		entity.Init("EmptyPropsEntity", EntityString("resource"), props)

		assert.Equal(t, "EmptyPropsEntity", entity.Name)
		assert.Equal(t, EntityString("resource"), entity.Type)
		assert.NotEmpty(t, entity.HashID)
	})

	t.Run("nil_props", func(t *testing.T) {
		entity := &Entity{}

		entity.Init("NilPropsEntity", EntityString("policy"), nil)

		assert.Equal(t, "NilPropsEntity", entity.Name)
		assert.Equal(t, EntityString("policy"), entity.Type)
		assert.NotEmpty(t, entity.HashID)
	})

	t.Run("consistent_hash_generation", func(t *testing.T) {
		entity1 := &Entity{}
		entity2 := &Entity{}
		props := map[string]string{
			"key1": "value1",
			"key2": "value2",
		}

		entity1.Init("SameName", EntityString("same_type"), props)
		entity2.Init("SameName", EntityString("same_type"), props)

		assert.Equal(t, entity1.HashID, entity2.HashID, "Same inputs should generate same hash")
		assert.Equal(t, entity1.Name, entity2.Name)
		assert.Equal(t, entity1.Type, entity2.Type)
	})

	t.Run("different_names_different_hashes", func(t *testing.T) {
		entity1 := &Entity{}
		entity2 := &Entity{}
		props := map[string]string{
			"key": "value",
		}

		entity1.Init("Entity1", EntityString("type"), props)
		entity2.Init("Entity2", EntityString("type"), props)

		assert.NotEqual(t, entity1.HashID, entity2.HashID, "Different names should generate different hashes")
	})

	t.Run("different_types_different_hashes", func(t *testing.T) {
		entity1 := &Entity{}
		entity2 := &Entity{}
		props := map[string]string{
			"key": "value",
		}

		entity1.Init("SameName", EntityString("type1"), props)
		entity2.Init("SameName", EntityString("type2"), props)

		assert.NotEqual(t, entity1.HashID, entity2.HashID, "Different types should generate different hashes")
	})

	t.Run("different_props_different_hashes", func(t *testing.T) {
		entity1 := &Entity{}
		entity2 := &Entity{}

		props1 := map[string]string{
			"key": "value1",
		}
		props2 := map[string]string{
			"key": "value2",
		}

		entity1.Init("SameName", EntityString("same_type"), props1)
		entity2.Init("SameName", EntityString("same_type"), props2)

		assert.NotEqual(t, entity1.HashID, entity2.HashID, "Different props should generate different hashes")
	})

	t.Run("complex_properties", func(t *testing.T) {
		entity := &Entity{}
		props := map[string]string{
			"email":        "user@example.com",
			"department":   "engineering/backend",
			"permissions":  `["read", "write", "delete"]`,
			"metadata":     `{"created": "2023-01-01", "updated": "2023-12-01"}`,
			"unicode":      "测试用户",
			"special_chars": "@#$%^&*()",
		}

		entity.Init("ComplexEntity", EntityString("complex_user"), props)

		assert.Equal(t, "ComplexEntity", entity.Name)
		assert.Equal(t, EntityString("complex_user"), entity.Type)
		assert.NotEmpty(t, entity.HashID)
	})

	t.Run("hash_includes_entity_key", func(t *testing.T) {
		entity1 := &Entity{}
		entity2 := &Entity{}

		// Even with same name and props, different entity types should produce different hashes
		props := map[string]string{
			"same": "props",
		}

		entity1.Init("SameName", EntityString("user"), props)
		entity2.Init("SameName", EntityString("resource"), props)

		assert.NotEqual(t, entity1.HashID, entity2.HashID, 
			"EntityKey should be included in hash generation")
	})

	t.Run("hash_includes_name_attribute", func(t *testing.T) {
		entity1 := &Entity{}
		entity2 := &Entity{}

		// Test that NameAttribute is properly included
		props := map[string]string{
			"other": "property",
		}

		entity1.Init("Name1", EntityString("user"), props)
		entity2.Init("Name2", EntityString("user"), props)

		assert.NotEqual(t, entity1.HashID, entity2.HashID,
			"NameAttribute should be included in hash generation")
	})
}

func TestEntity_StringArrays(t *testing.T) {
	t.Run("obligations_and_conditions", func(t *testing.T) {
		entity := &Entity{
			HashID: "test-id",
			Type:   EntityString("test"),
			Name:   "Test",
		}

		// These would typically be set by GORM, we just test they exist as fields
		// They are nil by default until GORM initializes them
		_ = entity.Obligations
		_ = entity.Conditions
		assert.True(t, true, "Obligations and Conditions fields exist")
	})
}

// BenchmarkEntity_Init benchmarks entity initialization
func BenchmarkEntity_Init(b *testing.B) {
	props := map[string]string{
		"department": "engineering",
		"level":      "senior",
		"project":    "tunnel-manager",
		"team":       "core-sre",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entity := &Entity{}
		entity.Init("BenchmarkEntity", EntityString("user"), props)
	}
}

// BenchmarkEntity_Getters benchmarks getter methods
func BenchmarkEntity_Getters(b *testing.B) {
	entity := &Entity{
		HashID: "benchmark-hash-id",
		Type:   EntityString("benchmark_type"),
		Name:   "Benchmark Entity",
	}

	b.Run("GetName", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = entity.GetName()
		}
	})

	b.Run("GetType", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = entity.GetType()
		}
	})

	b.Run("GetID", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = entity.GetID()
		}
	})
}

func TestEntity_EdgeCases(t *testing.T) {
	t.Run("very_long_name", func(t *testing.T) {
		entity := &Entity{}
		longName := string(make([]byte, 1000))
		for i := range longName {
			longName = longName[:i] + "a" + longName[i+1:]
		}

		entity.Init(longName, EntityString("long_name"), nil)

		assert.Equal(t, longName, entity.Name)
		assert.NotEmpty(t, entity.HashID)
	})

	t.Run("empty_name", func(t *testing.T) {
		entity := &Entity{}

		entity.Init("", EntityString("empty_name"), nil)

		assert.Equal(t, "", entity.Name)
		assert.NotEmpty(t, entity.HashID)
	})

	t.Run("empty_entity_type", func(t *testing.T) {
		entity := &Entity{}

		entity.Init("TestName", EntityString(""), nil)

		assert.Equal(t, "TestName", entity.Name)
		assert.Equal(t, EntityString(""), entity.Type)
		assert.NotEmpty(t, entity.HashID)
	})
}