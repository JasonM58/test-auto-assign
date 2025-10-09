package token

import (
	"context"
	"runtime"

	"github.com/alexedwards/argon2id"
)

func (a *TokenService) GenerateSecureToken(ctx context.Context, text string) (hash string, err error) {
	p := a.getParams()

	hash, err = argon2id.CreateHash(text, p)
	if err != nil {
		return "", err
	}

	return hash, nil
}

func (a *TokenService) ComparePlainTextAndCipherText(ctx context.Context, plainText, cipherText string) (match bool, err error) {
	match, err = argon2id.ComparePasswordAndHash(plainText, cipherText)
	if err != nil {
		return false, err
	}

	return match, nil
}

func (a *TokenService) getParams() *argon2id.Params {
	return &argon2id.Params{
		Memory:      128 * 1024,
		Iterations:  4,
		Parallelism: uint8(runtime.NumCPU()),
		SaltLength:  16,
		KeyLength:   32,
	}
}
