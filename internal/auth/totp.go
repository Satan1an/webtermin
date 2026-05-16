package auth

import (
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

func GenerateTOTPSecret(issuer, account string) (*otp.Key, error) {
	return totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: account,
	})
}

func ValidateTOTP(secret, code string) bool {
	if secret == "" {
		return false
	}
	return totp.Validate(code, secret)
}
