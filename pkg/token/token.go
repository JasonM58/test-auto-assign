package token

type (
	TokenService struct {
		jwtSecret string
	}
)

func NewTokenService(jwtSecret string) TokenService {
	return TokenService{
		jwtSecret: jwtSecret,
	}
}
