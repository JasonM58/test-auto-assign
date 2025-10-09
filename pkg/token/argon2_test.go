package token_test

import (
	"context"
	"testing"

	"github.com/ionextai/git-scrapper/pkg/token"
)

func Test_utilsSvc_GenerateSecureToken(t *testing.T) {
	type args struct {
		text string
	}
	tests := []struct {
		name    string
		a       *token.TokenService
		args    args
		wantErr bool
	}{
		{
			name:    "valid",
			a:       &token.TokenService{},
			args:    args{text: "imready"},
			wantErr: false,
		},
		{
			name:    "blank text",
			a:       &token.TokenService{},
			args:    args{text: ""},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &token.TokenService{}
			_, err := a.GenerateSecureToken(context.Background(), tt.args.text)

			if (err != nil) != tt.wantErr {
				t.Errorf("utilsSvc.GenerateSecureToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func Test_utilsSvc_CompareCipherTextAndPlainText(t *testing.T) {
	type args struct {
		plainText  string
		cipherText string
	}
	tests := []struct {
		name      string
		a         *token.TokenService
		args      args
		wantMatch bool
		wantErr   bool
	}{
		{
			name:      "match",
			a:         &token.TokenService{},
			args:      args{plainText: "imready", cipherText: "$argon2id$v=19$m=131072,t=4,p=22$3i34Su6Go/28slmhh5mjNQ$3SXZi7qjmutPg80aMPDBz6KKcaXTqapRafF7OdL95Lg"},
			wantMatch: true,
			wantErr:   false,
		},
		{
			name:      "not match",
			a:         &token.TokenService{},
			args:      args{plainText: "imready2", cipherText: "$argon2id$v=19$m=131072,t=4,p=22$3i34Su6Go/28slmhh5mjNQ$3SXZi7qjmutPg80aMPDBz6KKcaXTqapRafF7OdL95Lg"},
			wantMatch: false,
			wantErr:   false,
		},
		{
			name:      "blank text",
			a:         &token.TokenService{},
			args:      args{plainText: "", cipherText: "$argon2id$v=19$m=131072,t=4,p=22$3i34Su6Go/28slmhh5mjNQ$3SXZi7qjmutPg80aMPDBz6KKcaXTqapRafF7OdL95Lg"},
			wantMatch: false,
			wantErr:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &token.TokenService{}
			gotMatch, err := a.ComparePlainTextAndCipherText(context.Background(), tt.args.plainText, tt.args.cipherText)
			if (err != nil) != tt.wantErr {
				t.Errorf("utilsSvc.CompareCipherTextAndPlainText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotMatch != tt.wantMatch {
				t.Errorf("utilsSvc.CompareCipherTextAndPlainText() = %v, want %v", gotMatch, tt.wantMatch)
			}
		})
	}
}

func Test_utilsSvc_CompareOriginalAndComparedText(t *testing.T) {
	type args struct {
		originalText string
		comparedText string
	}
	tests := []struct {
		name      string
		a         *token.TokenService
		args      args
		wantMatch bool
		wantErr   bool
	}{
		{
			name:      "match",
			a:         &token.TokenService{},
			args:      args{originalText: "imready99", comparedText: "imready99"},
			wantMatch: true,
			wantErr:   false,
		},
		{
			name:      "not match",
			a:         &token.TokenService{},
			args:      args{originalText: "imready2", comparedText: "imready"},
			wantMatch: false,
			wantErr:   false,
		},
		{
			name:      "not match because whitespace",
			a:         &token.TokenService{},
			args:      args{originalText: "imready99", comparedText: "imready99 "},
			wantMatch: false,
			wantErr:   false,
		},
		{
			name:      "compared to blank",
			a:         &token.TokenService{},
			args:      args{originalText: "imready99", comparedText: ""},
			wantMatch: false,
			wantErr:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &token.TokenService{}
			hash, err := a.GenerateSecureToken(context.Background(), tt.args.originalText)

			if (err != nil) != tt.wantErr {
				t.Errorf("utilsSvc.GenerateSecureToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			a = &token.TokenService{}
			gotMatch, err := a.ComparePlainTextAndCipherText(context.Background(), tt.args.comparedText, hash)
			if (err != nil) != tt.wantErr {
				t.Errorf("utilsSvc.CompareCipherTextAndPlainText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotMatch != tt.wantMatch {
				t.Errorf("utilsSvc.CompareCipherTextAndPlainText() = %v, want %v", gotMatch, tt.wantMatch)
			}

		})
	}
}
