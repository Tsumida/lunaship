package infra

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecretStrMask(t *testing.T) {

	t.Run("size <= input.size", func(t *testing.T) {
		assert.Equal(
			t,
			"****EF",
			SecretStrMask("ABCDEF", 2),
		)
	})

	t.Run("size > input.size", func(t *testing.T) {
		assert.Equal(
			t,
			"******",
			SecretStrMask("ABCDEF", 7),
		)
	})

	t.Run("size = 0", func(t *testing.T) {
		assert.Equal(
			t,
			"ABCDEF",
			SecretStrMask("ABCDEF", 0),
		)
	})

	t.Run("Non-ascii char", func(t *testing.T) {
		assert.Equal(
			t,
			"****世界",
			SecretStrMask("你好, 世界", 2),
		)
	})
}
