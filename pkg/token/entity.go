package token

type (
	TokenType string
)

const (
	Authorization         string = "Authorization"
	BearerScheme          string = "Bearer "
	BearerTokenType       string = "AccessTokenType"
	RefreshTokenType      string = "RefreshTokenType"
	ResendEmailTokenType  string = "ResendEmailTokenType"
	EmailConfirmationType string = "EmailConfirmationType"
	RefreshTokenHeader    string = "X-Refresh-Token"

	AuthType        TokenType = "AuthType"
	ResendEmailType TokenType = "ResendEmailType"
)
