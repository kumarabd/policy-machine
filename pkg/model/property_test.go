package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPropertyString(t *testing.T) {
	t.Run("string_conversion", func(t *testing.T) {
		ps := PropertyString("test_property")
		assert.Equal(t, "test_property", ps.String())
	})

	t.Run("empty_string", func(t *testing.T) {
		ps := PropertyString("")
		assert.Equal(t, "", ps.String())
	})

	t.Run("property_constants", func(t *testing.T) {
		assert.Equal(t, "property_key", PropertyKey.String())
		assert.Equal(t, "property_value", PropertyValue.String())
	})
}

func TestMapToProperty(t *testing.T) {
	t.Run("basic_map_conversion", func(t *testing.T) {
		input := map[string]string{
			"name":        "john_doe",
			"email":       "john@example.com",
			"department":  "engineering",
		}

		properties := MapToProperty(input)

		assert.Len(t, properties, 3)
		
		// Verify all properties have IDs generated
		for _, prop := range properties {
			assert.NotEmpty(t, prop.ID, "Property ID should be generated")
			assert.NotEmpty(t, prop.Key, "Property key should be set")
			assert.NotEmpty(t, prop.Value, "Property value should be set")
		}

		// Verify all input keys are present
		keys := make(map[string]bool)
		for _, prop := range properties {
			keys[prop.Key] = true
		}
		assert.True(t, keys["name"])
		assert.True(t, keys["email"])
		assert.True(t, keys["department"])
	})

	t.Run("empty_map", func(t *testing.T) {
		input := make(map[string]string)

		properties := MapToProperty(input)

		assert.Len(t, properties, 0)
		assert.NotNil(t, properties, "Should return empty slice, not nil")
	})

	t.Run("nil_map", func(t *testing.T) {
		properties := MapToProperty(nil)

		assert.Len(t, properties, 0)
		assert.NotNil(t, properties, "Should return empty slice, not nil")
	})

	t.Run("single_property", func(t *testing.T) {
		input := map[string]string{
			"role": "admin",
		}

		properties := MapToProperty(input)

		assert.Len(t, properties, 1)
		assert.Equal(t, "role", properties[0].Key)
		assert.Equal(t, "admin", properties[0].Value)
		assert.NotEmpty(t, properties[0].ID)
	})

	t.Run("properties_with_empty_values", func(t *testing.T) {
		input := map[string]string{
			"name":        "alice",
			"middle_name": "",
			"last_name":   "smith",
		}

		properties := MapToProperty(input)

		assert.Len(t, properties, 3)
		
		// Find the empty value property
		var emptyProp *Property
		for _, prop := range properties {
			if prop.Key == "middle_name" {
				emptyProp = prop
				break
			}
		}
		
		assert.NotNil(t, emptyProp)
		assert.Equal(t, "", emptyProp.Value)
		assert.NotEmpty(t, emptyProp.ID, "Should still generate ID for empty values")
	})

	t.Run("properties_with_special_characters", func(t *testing.T) {
		input := map[string]string{
			"email":    "user@example.com",
			"path":     "/home/user/documents",
			"json":     `{"key": "value", "nested": {"inner": "data"}}`,
			"unicode":  "测试用户",
			"special":  "@#$%^&*()",
		}

		properties := MapToProperty(input)

		assert.Len(t, properties, 5)
		
		for _, prop := range properties {
			assert.NotEmpty(t, prop.ID)
			assert.NotEmpty(t, prop.Key)
			// Value can be empty, so just check it's not nil
			assert.NotNil(t, prop.Value)
		}
	})

	t.Run("consistent_id_generation", func(t *testing.T) {
		input := map[string]string{
			"user": "alice",
			"role": "admin",
		}

		properties1 := MapToProperty(input)
		properties2 := MapToProperty(input)

		assert.Len(t, properties1, 2)
		assert.Len(t, properties2, 2)

		// Create maps for easier comparison
		props1Map := make(map[string]string)
		props2Map := make(map[string]string)

		for _, prop := range properties1 {
			props1Map[prop.Key] = prop.ID
		}
		for _, prop := range properties2 {
			props2Map[prop.Key] = prop.ID
		}

		// Same input should generate same IDs
		assert.Equal(t, props1Map["user"], props2Map["user"])
		assert.Equal(t, props1Map["role"], props2Map["role"])
	})

	t.Run("different_values_different_ids", func(t *testing.T) {
		input1 := map[string]string{"key": "value1"}
		input2 := map[string]string{"key": "value2"}

		properties1 := MapToProperty(input1)
		properties2 := MapToProperty(input2)

		assert.NotEqual(t, properties1[0].ID, properties2[0].ID, 
			"Different values should generate different IDs")
	})
}

func TestPropertyToMap(t *testing.T) {
	t.Run("basic_property_conversion", func(t *testing.T) {
		properties := []*Property{
			{ID: "id1", Key: "name", Value: "alice"},
			{ID: "id2", Key: "email", Value: "alice@example.com"},
			{ID: "id3", Key: "role", Value: "admin"},
		}

		result := PropertyToMap(properties)

		assert.Len(t, result, 3)
		assert.Equal(t, "alice", result["name"])
		assert.Equal(t, "alice@example.com", result["email"])
		assert.Equal(t, "admin", result["role"])
	})

	t.Run("empty_properties", func(t *testing.T) {
		properties := []*Property{}

		result := PropertyToMap(properties)

		assert.Len(t, result, 0)
		assert.NotNil(t, result, "Should return empty map, not nil")
	})

	t.Run("nil_properties", func(t *testing.T) {
		result := PropertyToMap(nil)

		assert.Len(t, result, 0)
		assert.NotNil(t, result, "Should return empty map, not nil")
	})

	t.Run("single_property", func(t *testing.T) {
		properties := []*Property{
			{ID: "single-id", Key: "status", Value: "active"},
		}

		result := PropertyToMap(properties)

		assert.Len(t, result, 1)
		assert.Equal(t, "active", result["status"])
	})

	t.Run("properties_with_empty_values", func(t *testing.T) {
		properties := []*Property{
			{ID: "id1", Key: "name", Value: "bob"},
			{ID: "id2", Key: "middle_name", Value: ""},
			{ID: "id3", Key: "last_name", Value: "smith"},
		}

		result := PropertyToMap(properties)

		assert.Len(t, result, 3)
		assert.Equal(t, "bob", result["name"])
		assert.Equal(t, "", result["middle_name"])
		assert.Equal(t, "smith", result["last_name"])
	})

	t.Run("duplicate_keys_last_wins", func(t *testing.T) {
		properties := []*Property{
			{ID: "id1", Key: "role", Value: "user"},
			{ID: "id2", Key: "role", Value: "admin"}, // Duplicate key
		}

		result := PropertyToMap(properties)

		assert.Len(t, result, 1)
		assert.Equal(t, "admin", result["role"], "Last value should win for duplicate keys")
	})

	t.Run("properties_with_special_characters", func(t *testing.T) {
		properties := []*Property{
			{ID: "id1", Key: "email", Value: "user@example.com"},
			{ID: "id2", Key: "path", Value: "/home/user/docs"},
			{ID: "id3", Key: "json", Value: `{"nested": "value"}`},
			{ID: "id4", Key: "unicode", Value: "用戶"},
		}

		result := PropertyToMap(properties)

		assert.Len(t, result, 4)
		assert.Equal(t, "user@example.com", result["email"])
		assert.Equal(t, "/home/user/docs", result["path"])
		assert.Equal(t, `{"nested": "value"}`, result["json"])
		assert.Equal(t, "用戶", result["unicode"])
	})
}

func TestMapToProperty_PropertyToMap_RoundTrip(t *testing.T) {
	t.Run("round_trip_conversion", func(t *testing.T) {
		original := map[string]string{
			"user_id":    "12345",
			"username":   "test_user",
			"email":      "test@example.com",
			"department": "engineering",
			"level":      "senior",
		}

		// Convert to properties and back
		properties := MapToProperty(original)
		result := PropertyToMap(properties)

		assert.Equal(t, original, result, "Round trip should preserve all data")
	})

	t.Run("round_trip_with_empty_values", func(t *testing.T) {
		original := map[string]string{
			"name":        "test",
			"middle_name": "",
			"description": "test user",
		}

		properties := MapToProperty(original)
		result := PropertyToMap(properties)

		assert.Equal(t, original, result)
	})

	t.Run("round_trip_with_special_characters", func(t *testing.T) {
		original := map[string]string{
			"email":   "user+tag@example.com",
			"path":    "/path/with spaces/file.txt",
			"unicode": "测试 🚀 émojì",
			"json":    `{"key": "value with spaces", "number": 123}`,
		}

		properties := MapToProperty(original)
		result := PropertyToMap(properties)

		assert.Equal(t, original, result)
	})
}

// BenchmarkMapToProperty benchmarks map to property conversion
func BenchmarkMapToProperty(b *testing.B) {
	input := map[string]string{
		"user_id":    "12345",
		"username":   "benchmark_user",
		"email":      "bench@example.com",
		"department": "engineering",
		"role":       "developer",
		"level":      "senior",
		"team":       "backend",
		"project":    "tunnel-manager",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = MapToProperty(input)
	}
}

// BenchmarkPropertyToMap benchmarks property to map conversion
func BenchmarkPropertyToMap(b *testing.B) {
	properties := []*Property{
		{ID: "id1", Key: "user_id", Value: "12345"},
		{ID: "id2", Key: "username", Value: "benchmark_user"},
		{ID: "id3", Key: "email", Value: "bench@example.com"},
		{ID: "id4", Key: "department", Value: "engineering"},
		{ID: "id5", Key: "role", Value: "developer"},
		{ID: "id6", Key: "level", Value: "senior"},
		{ID: "id7", Key: "team", Value: "backend"},
		{ID: "id8", Key: "project", Value: "tunnel-manager"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = PropertyToMap(properties)
	}
}

// BenchmarkRoundTrip benchmarks the complete round trip
func BenchmarkRoundTrip(b *testing.B) {
	input := map[string]string{
		"user_id":  "12345",
		"username": "benchmark_user",
		"email":    "bench@example.com",
		"role":     "developer",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		properties := MapToProperty(input)
		_ = PropertyToMap(properties)
	}
}