package infra

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func TestParseToken(t *testing.T) {

	var (
		secret  = ""
		payload = map[string]interface{}{
			"sub":  "1234567890",
			"name": "John Doe",
			"iat":  int32(1516239022),
			"exp":  time.Now().Add(1 * time.Hour).Unix(),
		}
	)

	t.Run("sign-jwt", func(tt *testing.T) {
		tk, err := SignJWT(payload)
		assert.NoError(tt, err)
		tt.Log(tk)

		secret = tk
	})

	t.Run("validate-jwt", func(tt *testing.T) {
		tk, err := ParseToken(secret)
		assert.NoError(tt, err)

		claim, ok := tk.Claims.(jwt.MapClaims)
		assert.True(tt, ok)

		assert.Equal(tt, "1234567890", claim["sub"].(string))
		assert.Equal(tt, "John Doe", claim["name"].(string))
		assert.Equal(tt, int32(1516239022), int32(claim["iat"].(float64)))
	})

}
