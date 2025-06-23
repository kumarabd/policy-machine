package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRelationshipString(t *testing.T) {
	t.Run("string_conversion", func(t *testing.T) {
		rs := RelationshipString("test_relationship")
		assert.Equal(t, "test_relationship", rs.String())
	})

	t.Run("empty_string", func(t *testing.T) {
		rs := RelationshipString("")
		assert.Equal(t, "", rs.String())
	})

	t.Run("relationship_key_constant", func(t *testing.T) {
		assert.Equal(t, "relationship", RelationshipKey.String())
	})
}

func TestRelationship_Getters(t *testing.T) {
	rel := &Relationship{
		HashID: "test-rel-123",
		FromID: "from-entity-id",
		ToID:   "to-entity-id",
		Type:   RelationshipString("assignment"),
	}

	t.Run("get_id", func(t *testing.T) {
		assert.Equal(t, "test-rel-123", rel.GetID())
	})

	t.Run("get_type", func(t *testing.T) {
		assert.Equal(t, RelationshipString("assignment"), rel.GetType())
		assert.Equal(t, "assignment", rel.GetType().String())
	})
}

func TestRelationship_Init(t *testing.T) {
	t.Run("basic_initialization", func(t *testing.T) {
		// Create source and target entities
		fromEntity := &Entity{}
		fromEntity.Init("SourceEntity", EntityString("user"), map[string]string{"role": "admin"})

		toEntity := &Entity{}
		toEntity.Init("TargetEntity", EntityString("resource"), map[string]string{"type": "document"})

		// Initialize relationship
		rel := &Relationship{}
		rel.Init(*fromEntity, *toEntity, RelationshipString("assignment"))

		assert.NotEmpty(t, rel.HashID, "HashID should be generated")
		assert.Equal(t, fromEntity.HashID, rel.FromID)
		assert.Equal(t, toEntity.HashID, rel.ToID)
		assert.Equal(t, RelationshipString("assignment"), rel.Type)
	})

	t.Run("consistent_hash_generation", func(t *testing.T) {
		// Create identical entities
		fromEntity1 := &Entity{}
		fromEntity1.Init("User1", EntityString("user"), map[string]string{"dept": "eng"})

		toEntity1 := &Entity{}
		toEntity1.Init("Resource1", EntityString("resource"), map[string]string{"type": "file"})

		fromEntity2 := &Entity{}
		fromEntity2.Init("User1", EntityString("user"), map[string]string{"dept": "eng"})

		toEntity2 := &Entity{}
		toEntity2.Init("Resource1", EntityString("resource"), map[string]string{"type": "file"})

		// Create relationships
		rel1 := &Relationship{}
		rel1.Init(*fromEntity1, *toEntity1, RelationshipString("assignment"))

		rel2 := &Relationship{}
		rel2.Init(*fromEntity2, *toEntity2, RelationshipString("assignment"))

		assert.Equal(t, rel1.HashID, rel2.HashID, "Same entities should generate same relationship hash")
		assert.Equal(t, rel1.FromID, rel2.FromID)
		assert.Equal(t, rel1.ToID, rel2.ToID)
		assert.Equal(t, rel1.Type, rel2.Type)
	})

	t.Run("different_types_different_hashes", func(t *testing.T) {
		fromEntity := &Entity{}
		fromEntity.Init("User", EntityString("user"), nil)

		toEntity := &Entity{}
		toEntity.Init("Resource", EntityString("resource"), nil)

		rel1 := &Relationship{}
		rel1.Init(*fromEntity, *toEntity, RelationshipString("assignment"))

		rel2 := &Relationship{}
		rel2.Init(*fromEntity, *toEntity, RelationshipString("association"))

		assert.NotEqual(t, rel1.HashID, rel2.HashID, "Different relationship types should generate different hashes")
		assert.Equal(t, rel1.FromID, rel2.FromID, "FromID should be the same")
		assert.Equal(t, rel1.ToID, rel2.ToID, "ToID should be the same")
		assert.NotEqual(t, rel1.Type, rel2.Type, "Types should be different")
	})

	t.Run("different_from_entities_different_hashes", func(t *testing.T) {
		fromEntity1 := &Entity{}
		fromEntity1.Init("User1", EntityString("user"), nil)

		fromEntity2 := &Entity{}
		fromEntity2.Init("User2", EntityString("user"), nil)

		toEntity := &Entity{}
		toEntity.Init("Resource", EntityString("resource"), nil)

		rel1 := &Relationship{}
		rel1.Init(*fromEntity1, *toEntity, RelationshipString("assignment"))

		rel2 := &Relationship{}
		rel2.Init(*fromEntity2, *toEntity, RelationshipString("assignment"))

		assert.NotEqual(t, rel1.HashID, rel2.HashID, "Different from entities should generate different hashes")
		assert.NotEqual(t, rel1.FromID, rel2.FromID)
		assert.Equal(t, rel1.ToID, rel2.ToID)
	})

	t.Run("different_to_entities_different_hashes", func(t *testing.T) {
		fromEntity := &Entity{}
		fromEntity.Init("User", EntityString("user"), nil)

		toEntity1 := &Entity{}
		toEntity1.Init("Resource1", EntityString("resource"), nil)

		toEntity2 := &Entity{}
		toEntity2.Init("Resource2", EntityString("resource"), nil)

		rel1 := &Relationship{}
		rel1.Init(*fromEntity, *toEntity1, RelationshipString("assignment"))

		rel2 := &Relationship{}
		rel2.Init(*fromEntity, *toEntity2, RelationshipString("assignment"))

		assert.NotEqual(t, rel1.HashID, rel2.HashID, "Different to entities should generate different hashes")
		assert.Equal(t, rel1.FromID, rel2.FromID)
		assert.NotEqual(t, rel1.ToID, rel2.ToID)
	})

	t.Run("complex_entities", func(t *testing.T) {
		fromEntity := &Entity{}
		fromProps := map[string]string{
			"email":      "user@example.com",
			"department": "engineering",
			"role":       "senior",
			"permissions": `["read", "write"]`,
		}
		fromEntity.Init("ComplexUser", EntityString("user"), fromProps)

		toEntity := &Entity{}
		toProps := map[string]string{
			"filename":    "important-doc.pdf",
			"path":        "/secure/documents/",
			"sensitivity": "confidential",
			"owner":       "admin",
		}
		toEntity.Init("SecureDocument", EntityString("document"), toProps)

		rel := &Relationship{}
		rel.Init(*fromEntity, *toEntity, RelationshipString("access_control"))

		assert.NotEmpty(t, rel.HashID)
		assert.Equal(t, fromEntity.HashID, rel.FromID)
		assert.Equal(t, toEntity.HashID, rel.ToID)
		assert.Equal(t, RelationshipString("access_control"), rel.Type)
	})

	t.Run("self_relationship", func(t *testing.T) {
		entity := &Entity{}
		entity.Init("SelfEntity", EntityString("user"), map[string]string{"type": "recursive"})

		rel := &Relationship{}
		rel.Init(*entity, *entity, RelationshipString("self_reference"))

		assert.NotEmpty(t, rel.HashID)
		assert.Equal(t, entity.HashID, rel.FromID)
		assert.Equal(t, entity.HashID, rel.ToID)
		assert.Equal(t, rel.FromID, rel.ToID, "Self-relationship should have same from and to IDs")
	})
}

func TestRelationship_StructureFields(t *testing.T) {
	t.Run("obligations_and_conditions", func(t *testing.T) {
		rel := &Relationship{
			HashID: "test-id",
			FromID: "from-id",
			ToID:   "to-id",
			Type:   RelationshipString("test"),
		}

		// These would typically be set by GORM or application logic
		// They are nil by default until GORM initializes them
		_ = rel.Obligations
		_ = rel.Conditions
		assert.True(t, true, "Obligations and Conditions fields exist")
	})

	t.Run("indexing_fields", func(t *testing.T) {
		// Test that indexed fields are properly set
		fromEntity := &Entity{}
		fromEntity.Init("User", EntityString("user"), nil)

		toEntity := &Entity{}
		toEntity.Init("Resource", EntityString("resource"), nil)

		rel := &Relationship{}
		rel.Init(*fromEntity, *toEntity, RelationshipString("test"))

		// Verify indexed fields are set correctly
		assert.NotEmpty(t, rel.FromID, "FromID should be set for indexing")
		assert.NotEmpty(t, rel.ToID, "ToID should be set for indexing")
		assert.NotEmpty(t, rel.Type, "Type should be set for indexing")
	})
}

func TestRelationship_EdgeCases(t *testing.T) {
	t.Run("empty_entity_names", func(t *testing.T) {
		fromEntity := &Entity{}
		fromEntity.Init("", EntityString("user"), nil)

		toEntity := &Entity{}
		toEntity.Init("", EntityString("resource"), nil)

		rel := &Relationship{}
		rel.Init(*fromEntity, *toEntity, RelationshipString("assignment"))

		assert.NotEmpty(t, rel.HashID)
		assert.Equal(t, fromEntity.HashID, rel.FromID)
		assert.Equal(t, toEntity.HashID, rel.ToID)
	})

	t.Run("empty_relationship_type", func(t *testing.T) {
		fromEntity := &Entity{}
		fromEntity.Init("User", EntityString("user"), nil)

		toEntity := &Entity{}
		toEntity.Init("Resource", EntityString("resource"), nil)

		rel := &Relationship{}
		rel.Init(*fromEntity, *toEntity, RelationshipString(""))

		assert.NotEmpty(t, rel.HashID)
		assert.Equal(t, RelationshipString(""), rel.Type)
	})

	t.Run("very_long_relationship_type", func(t *testing.T) {
		fromEntity := &Entity{}
		fromEntity.Init("User", EntityString("user"), nil)

		toEntity := &Entity{}
		toEntity.Init("Resource", EntityString("resource"), nil)

		longType := RelationshipString(string(make([]byte, 1000)))
		for i := range longType {
			longType = RelationshipString(string(longType)[:i] + "a" + string(longType)[i+1:])
		}

		rel := &Relationship{}
		rel.Init(*fromEntity, *toEntity, longType)

		assert.NotEmpty(t, rel.HashID)
		assert.Equal(t, longType, rel.Type)
	})
}

// BenchmarkRelationship_Init benchmarks relationship initialization
func BenchmarkRelationship_Init(b *testing.B) {
	fromEntity := &Entity{}
	fromEntity.Init("BenchUser", EntityString("user"), map[string]string{"dept": "eng"})

	toEntity := &Entity{}
	toEntity.Init("BenchResource", EntityString("resource"), map[string]string{"type": "file"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rel := &Relationship{}
		rel.Init(*fromEntity, *toEntity, RelationshipString("assignment"))
	}
}

// BenchmarkRelationship_Getters benchmarks getter methods
func BenchmarkRelationship_Getters(b *testing.B) {
	rel := &Relationship{
		HashID: "benchmark-rel-id",
		FromID: "from-id",
		ToID:   "to-id",
		Type:   RelationshipString("benchmark_type"),
	}

	b.Run("GetID", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = rel.GetID()
		}
	})

	b.Run("GetType", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = rel.GetType()
		}
	})
}

func TestRelationship_Integration(t *testing.T) {
	t.Run("relationship_chain", func(t *testing.T) {
		// Create a chain of relationships: User -> Role -> Permission -> Resource
		user := &Entity{}
		user.Init("alice", EntityString("user"), map[string]string{"dept": "engineering"})

		role := &Entity{}
		role.Init("developer", EntityString("role"), map[string]string{"level": "senior"})

		permission := &Entity{}
		permission.Init("write_access", EntityString("permission"), map[string]string{"scope": "project"})

		resource := &Entity{}
		resource.Init("source_code", EntityString("resource"), map[string]string{"repo": "tunnel-manager"})

		// Create relationships
		userToRole := &Relationship{}
		userToRole.Init(*user, *role, RelationshipString("assignment"))

		roleToPermission := &Relationship{}
		roleToPermission.Init(*role, *permission, RelationshipString("includes"))

		permissionToResource := &Relationship{}
		permissionToResource.Init(*permission, *resource, RelationshipString("grants_access"))

		// Verify the chain
		assert.Equal(t, user.HashID, userToRole.FromID)
		assert.Equal(t, role.HashID, userToRole.ToID)

		assert.Equal(t, role.HashID, roleToPermission.FromID)
		assert.Equal(t, permission.HashID, roleToPermission.ToID)

		assert.Equal(t, permission.HashID, permissionToResource.FromID)
		assert.Equal(t, resource.HashID, permissionToResource.ToID)

		// All relationships should have unique IDs
		assert.NotEqual(t, userToRole.HashID, roleToPermission.HashID)
		assert.NotEqual(t, roleToPermission.HashID, permissionToResource.HashID)
		assert.NotEqual(t, userToRole.HashID, permissionToResource.HashID)
	})
}