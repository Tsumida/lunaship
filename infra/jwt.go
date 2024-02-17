package infra

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/bufbuild/connect-go"
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

// Panic if svcJwtSecret is invalid.
func MiddlewareJWTAuth(
	svcJwtSecret string,
) connect.UnaryInterceptorFunc {

	if len(strings.TrimSpace(svcJwtSecret)) == 0 {
		panic("invalid jwt secret")
	} else {
		fmt.Printf("load jwt secret:%s\n", SecretStrMask(svcJwtSecret, 8))
	}

	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {
			headers := req.Header()
			// Authorization: Bearer xxxx
			jwtToken := strings.TrimSpace(strings.TrimPrefix(headers.Get("Authorization"), "Bearer "))
			_, err := ParseToken(jwtToken)
			if err != nil {
				return nil, err
			}

			return next(ctx, req)
		})
	}
	return connect.UnaryInterceptorFunc(interceptor)
}
