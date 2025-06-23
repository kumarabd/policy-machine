package utils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateMD5ID(t *testing.T) {
	t.Run("simple_map", func(t *testing.T) {
		intent := map[string]string{
			"key1": "value1",
			"key2": "value2",
		}

		id, err := GenerateMD5ID(intent)
		require.NoError(t, err)
		assert.NotEmpty(t, id)
		assert.Equal(t, 32, len(id), "MD5 hash should be 32 characters")
	})

	t.Run("empty_map", func(t *testing.T) {
		intent := make(map[string]string)

		id, err := GenerateMD5ID(intent)
		require.NoError(t, err)
		assert.NotEmpty(t, id)
		assert.Equal(t, 32, len(id))
	})

	t.Run("consistent_output", func(t *testing.T) {
		intent := map[string]string{
			"user":     "alice",
			"action":   "read",
			"resource": "file1",
		}

		id1, err1 := GenerateMD5ID(intent)
		require.NoError(t, err1)

		id2, err2 := GenerateMD5ID(intent)
		require.NoError(t, err2)

		assert.Equal(t, id1, id2, "Same input should generate same hash")
	})

	t.Run("different_inputs_different_outputs", func(t *testing.T) {
		intent1 := map[string]string{
			"key": "value1",
		}
		intent2 := map[string]string{
			"key": "value2",
		}

		id1, err1 := GenerateMD5ID(intent1)
		require.NoError(t, err1)

		id2, err2 := GenerateMD5ID(intent2)
		require.NoError(t, err2)

		assert.NotEqual(t, id1, id2, "Different inputs should generate different hashes")
	})

	t.Run("special_characters", func(t *testing.T) {
		intent := map[string]string{
			"special": "äöüß@#$%^&*()",
			"unicode": "🚀💡🔥",
			"json":    `{"nested": "value"}`,
		}

		id, err := GenerateMD5ID(intent)
		require.NoError(t, err)
		assert.NotEmpty(t, id)
		assert.Equal(t, 32, len(id))
	})
}

func TestGenerateJWTID(t *testing.T) {
	t.Run("simple_map", func(t *testing.T) {
		intent := map[string]string{
			"sub": "user123",
			"iss": "test-issuer",
		}
		salt := "test"

		token, err := GenerateJWTID(intent, salt)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		// JWT should have 3 parts separated by dots
		parts := strings.Split(token, ".")
		assert.Equal(t, 3, len(parts), "JWT should have 3 parts (header.payload.signature)")
	})

	t.Run("empty_map", func(t *testing.T) {
		intent := make(map[string]string)
		salt := "test"

		token, err := GenerateJWTID(intent, salt)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		parts := strings.Split(token, ".")
		assert.Equal(t, 3, len(parts))
	})

	t.Run("consistent_output", func(t *testing.T) {
		intent := map[string]string{
			"user":     "alice",
			"resource": "document1",
			"action":   "write",
		}
		salt := "test"

		token1, err1 := GenerateJWTID(intent, salt)
		require.NoError(t, err1)

		token2, err2 := GenerateJWTID(intent, salt)
		require.NoError(t, err2)

		assert.Equal(t, token1, token2, "Same input should generate same JWT")
	})

	t.Run("different_inputs_different_outputs", func(t *testing.T) {
		intent1 := map[string]string{
			"user": "alice",
		}
		intent2 := map[string]string{
			"user": "bob",
		}
		salt := "test"

		token1, err1 := GenerateJWTID(intent1, salt)
		require.NoError(t, err1)

		token2, err2 := GenerateJWTID(intent2, salt)
		require.NoError(t, err2)

		assert.NotEqual(t, token1, token2, "Different inputs should generate different JWTs")
	})

	t.Run("large_payload", func(t *testing.T) {
		intent := make(map[string]string)
		for i := 0; i < 100; i++ {
			intent[string(rune('a'+i%26))+string(rune('0'+i%10))] = strings.Repeat("x", 50)
		}
		salt := "test"

		token, err := GenerateJWTID(intent, salt)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		parts := strings.Split(token, ".")
		assert.Equal(t, 3, len(parts))
	})

	t.Run("special_characters_in_claims", func(t *testing.T) {
		intent := map[string]string{
			"email":    "test@example.com",
			"role":     "admin/super-user",
			"metadata": `{"department": "engineering", "level": 5}`,
			"unicode":  "測試用戶",
		}
		salt := "test"

		token, err := GenerateJWTID(intent, salt)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		parts := strings.Split(token, ".")
		assert.Equal(t, 3, len(parts))
	})
}

func TestGenerateID_Comparison(t *testing.T) {
	t.Run("md5_vs_jwt_different_outputs", func(t *testing.T) {
		intent := map[string]string{
			"user":   "testuser",
			"action": "read",
		}
		salt := "test"

		md5ID, err1 := GenerateMD5ID(intent)
		require.NoError(t, err1)

		jwtID, err2 := GenerateJWTID(intent, salt)
		require.NoError(t, err2)

		assert.NotEqual(t, md5ID, jwtID, "MD5 and JWT should produce different formats")
		assert.Equal(t, 32, len(md5ID), "MD5 should be 32 chars")
		assert.True(t, len(jwtID) > 32, "JWT should be longer than MD5")
	})
}

// BenchmarkGenerateMD5ID benchmarks MD5 ID generation
func BenchmarkGenerateMD5ID(b *testing.B) {
	intent := map[string]string{
		"user":     "benchmarkuser",
		"resource": "resource123",
		"action":   "read",
		"tenant":   "default",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GenerateMD5ID(intent)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGenerateJWTID benchmarks JWT ID generation
func BenchmarkGenerateJWTID(b *testing.B) {
	intent := map[string]string{
		"user":     "benchmarkuser",
		"resource": "resource123",
		"action":   "read",
		"tenant":   "default",
	}
	salt := "test"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GenerateJWTID(intent, salt)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLargePayload benchmarks with large payloads
func BenchmarkLargePayload(b *testing.B) {
	// Create a large intent map
	intent := make(map[string]string)
	for i := 0; i < 100; i++ {
		intent[string(rune('a'+i%26))+string(rune('0'+i%10))] = strings.Repeat("data", 20)
	}
	salt := "test"

	b.Run("MD5", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := GenerateMD5ID(intent)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("JWT", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := GenerateJWTID(intent, salt)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
