package middleware

import (
	"context"
	"fmt"
	"strings"

	"github.com/bufbuild/connect-go"
	"github.com/tsumida/lunaship/infra"
	"github.com/tsumida/lunaship/infra/crypto"
)

// Panic if svcJwtSecret is invalid.
func MiddlewareJWTAuth(
	svcJwtSecret string,
) connect.UnaryInterceptorFunc {

	if len(strings.TrimSpace(svcJwtSecret)) == 0 {
		panic("invalid jwt secret")
	} else {
		fmt.Printf("load jwt secret:%s\n", infra.SecretStrMask(svcJwtSecret, 8))
	}

	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {
			headers := req.Header()
			// Authorization: Bearer xxxx
			jwtToken := strings.TrimSpace(strings.TrimPrefix(headers.Get("Authorization"), "Bearer "))
			_, err := crypto.ParseToken(jwtToken)
			if err != nil {
				return nil, err
			}

			return next(ctx, req)
		})
	}
	return connect.UnaryInterceptorFunc(interceptor)
}
