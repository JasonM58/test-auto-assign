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

// LoadPrivateKey reads and parses the PEM private key file
// func LoadPrivateKey(pemPath string) (*rsa.PrivateKey, error) {
// 	keyBytes, err := ioutil.ReadFile(pemPath)
// 	if err != nil {
// 		return nil, fmt.Errorf("cannot read private key file: %w", err)
// 	}

// 	block, _ := pem.Decode(keyBytes)
// 	if block == nil || block.Type != "RSA PRIVATE KEY" {
// 		return nil, fmt.Errorf("invalid PEM block or key type")
// 	}

// 	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(keyBytes)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to parse RSA private key: %w", err)
// 	}

// 	return privateKey, nil
// }

func LoadPrivateKey(input string) (*rsa.PrivateKey, error) {
	var pemBytes []byte

	// Handle both inline PEM and file path
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
		// Try PKCS8 format if PKCS1 fails
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



// GenerateJWT creates a signed JWT for GitHub App authentication
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

// GetInstallationToken returns an access token for the GitHub App installation
func (a *GitHubAuth) GetInstallationToken(ctx context.Context) (string, error) {
	jwtToken, err := a.GenerateJWT()
	if err != nil {
		return "", fmt.Errorf("failed to generate JWT: %w", err)
	}

	// Authenticate with JWT to list installations
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

	// Use the first installation found
	instID := installations[0].GetID()

	token, _, err := client.Apps.CreateInstallationToken(ctx, instID, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create installation token: %w", err)
	}

	return token.GetToken(), nil
}
