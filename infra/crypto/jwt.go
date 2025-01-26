package crypto

import (
	"errors"
	"os"

	jwt "github.com/golang-jwt/jwt/v5"
)

func ParseToken(tokenString string) (*jwt.Token, error) {
	return jwt.Parse(
		tokenString,
		func(t *jwt.Token) (interface{}, error) {
			// validate alg
			if m, ok := t.Method.(*jwt.SigningMethodHMAC); !ok || m.Name != "HS256" {
				return nil, errors.New("unexpected decoder or hash alg")
			}

			return []byte(os.Getenv("JWT_MAILBOX_KEY")), nil
		},
		jwt.WithExpirationRequired(),
	)
}

func SignJWT(payloadMap map[string]interface{}) (string, error) {
	tk := jwt.NewWithClaims(jwt.SigningMethodHS256, (jwt.MapClaims)(payloadMap))
	return tk.SignedString([]byte(os.Getenv("JWT_MAILBOX_KEY")))
}
