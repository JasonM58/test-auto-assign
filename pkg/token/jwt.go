package token

import (
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

type (
	JWTClaim struct {
		Sub   string
		Email string
		IAT   *jwt.NumericDate
		EXP   *jwt.NumericDate
		Name  string
		Type  string
	}

	JWTValidation struct {
		UserID string
		Type   string
		EXP    int64
		Email  string
	}
)

func (j *TokenService) JwtBuildAndSignJSON(jwtClaim JWTClaim) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   jwtClaim.Sub,
		"email": jwtClaim.Email,
		"iat":   jwtClaim.IAT,
		"exp":   jwtClaim.EXP,
		"name":  jwtClaim.Name,
		"type":  jwtClaim.Type,
	})

	tokenString, err := token.SignedString(j.secretKey())
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func (j *TokenService) JwtValidate(tokenString string) (JWTValidation, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return JWTValidation{}, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return j.secretKey(), nil
	})

	if token == nil {
		return JWTValidation{}, errors.New("token is blank")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return JWTValidation{}, errors.New("invalid token")
	}

	var userID string
	var tokenType string
	var expiredAt float64
	var userIDClaim = claims["sub"]
	var email string
	fetchedUserID, ok := userIDClaim.(string)
	if !ok {
		return JWTValidation{}, errors.New("invalid jwt on sub")
	}

	var claimTokenType = claims["type"]
	fetchedTokenType, ok := claimTokenType.(string)
	if !ok {
		return JWTValidation{}, errors.New("invalid jwt on type")
	}

	userID = fetchedUserID
	tokenType = fetchedTokenType

	var claimExp = claims["exp"]
	expiredAt, ok = claimExp.(float64)
	if !ok {
		return JWTValidation{}, errors.New("invalid jwt on exp")
	}

	email, ok = claims["email"].(string)
	if !ok {
		return JWTValidation{}, errors.New("invalid jwt on email")
	}

	return JWTValidation{
		UserID: userID, Type: tokenType, EXP: int64(expiredAt), Email: email,
	}, err
}

func (j *TokenService) secretKey() []byte {
	var secretKeyBytes = []byte(j.jwtSecret)
	return secretKeyBytes
}
