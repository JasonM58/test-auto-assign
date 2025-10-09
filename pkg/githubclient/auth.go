package githubclient

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/go-github/v55/github"
	"golang.org/x/oauth2"
)

type GitHubAuth struct {
	AppID      int64
	PrivateKey *rsa.PrivateKey
}

func LoadPrivateKey(input string) (*rsa.PrivateKey, error) {
	var pemBytes []byte

	if strings.HasPrefix(input, "-----BEGIN") {
		pemBytes = []byte(input)
	} else {
		data, err := os.ReadFile(input)
		if err != nil {
			return nil, err
		}
		pemBytes = data
	}

	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("failed to parse PEM block containing the key")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		pkcs8Key, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			return nil, err
		}
		switch k := pkcs8Key.(type) {
		case *rsa.PrivateKey:
			return k, nil
		default:
			return nil, errors.New("not an RSA private key")
		}
	}

	return key, nil
}



func (a *GitHubAuth) GenerateJWT() (string, error) {
	now := time.Now()
	claims := jwt.StandardClaims{
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(time.Minute * 10).Unix(),
		Issuer:    fmt.Sprintf("%d", a.AppID),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(a.PrivateKey)
}

func (a *GitHubAuth) GetInstallationToken(ctx context.Context) (string, error) {
	jwtToken, err := a.GenerateJWT()
	if err != nil {
		return "", fmt.Errorf("failed to generate JWT: %w", err)
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{TokenType: "Bearer", AccessToken: jwtToken})
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	installations, _, err := client.Apps.ListInstallations(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to list installations: %w", err)
	}

	if len(installations) == 0 {
		return "", fmt.Errorf("no installations found for this GitHub App")
	}

	instID := installations[0].GetID()

	token, _, err := client.Apps.CreateInstallationToken(ctx, instID, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create installation token: %w", err)
	}

	return token.GetToken(), nil
}
