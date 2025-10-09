package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/ionextai/git-scrapper/pkg/ctxhelper"
	"github.com/ionextai/git-scrapper/pkg/errorshelper"
	"github.com/ionextai/git-scrapper/pkg/jsonresponder"
	"github.com/ionextai/git-scrapper/pkg/logger"
	"github.com/ionextai/git-scrapper/pkg/token"
)

type (
	TokenServiceInterface interface {
		JwtValidate(tokenString string) (token.JWTValidation, error)
	}

	mdw struct {
		jwtSecret     string
		tokenService  TokenServiceInterface
		jsonResponder jsonresponder.JSONResponder
		logger        *slog.Logger
	}
)

var (
	mdwOnce sync.Once
	mdwVar  mdw
)

func NewMiddleware(jwtSecret string) mdw {
	mdwOnce.Do(func() {
		tokenSvc := token.NewTokenService(jwtSecret)
		loggerSvc := logger.NewJSONLogger()
		mdwVar = mdw{
			jwtSecret:     jwtSecret,
			tokenService:  &tokenSvc,
			jsonResponder: jsonresponder.NewDefaultJSONResponder(),
			logger:        loggerSvc,
		}
	})

	return mdwVar
}

func (m *mdw) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get(token.Authorization)
		authHeader = strings.TrimPrefix(authHeader, token.BearerScheme)
		ctx := r.Context()

		jwtValidation, err := m.authorizeToken(ctx, authHeader, token.BearerTokenType)
		if err != nil {
			var validationError *errorshelper.ValidationErrors
			if errors.As(err, &validationError) {
				m.jsonResponder.Error(w, validationError.ResponseCode(), validationError)
				return
			}
			m.jsonResponder.WriteSimpleError(w, http.StatusInternalServerError, errorshelper.InternalServerMsg)
			return
		}

		ctx = context.WithValue(ctx, ctxhelper.UserIDKey, jwtValidation.UserID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *mdw) authorizeToken(ctx context.Context, authToken string, tokenType string) (jwtValidation token.JWTValidation, err error) {
	jwtValidation, err = m.tokenService.JwtValidate(authToken)
	if err != nil {
		return jwtValidation, errorshelper.NewValidationError([]errorshelper.ValidationError{
			{
				Field: "Authorization", Message: http.StatusText(http.StatusUnauthorized), Type: errorshelper.NoAccess,
			},
		})
	}

	if jwtValidation.Type != tokenType {
		return jwtValidation, errorshelper.NewValidationError([]errorshelper.ValidationError{
			{
				Field: "Authorization", Message: http.StatusText(http.StatusUnauthorized), Type: errorshelper.NoAccess,
			},
		})
	}

	return jwtValidation, nil
}
