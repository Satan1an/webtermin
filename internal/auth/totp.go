package auth

import (
	"crypto/subtle"
	"time"

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

// totpPeriod matches the pquerna/otp default the codes are generated with.
const totpPeriod = 30

// consumeTOTP validates code for userID (same ±1 step skew as ValidateTOTP)
// and rejects a code from a time-step that user has already logged in with.
func (a *Service) consumeTOTP(userID int64, secret, code string) bool {
	if secret == "" || code == "" {
		return false
	}
	now := time.Now()
	for _, skew := range []int64{-1, 0, 1} {
		at := now.Add(time.Duration(skew*totpPeriod) * time.Second)
		want, err := totp.GenerateCode(secret, at)
		if err != nil {
			return false
		}
		if subtle.ConstantTimeCompare([]byte(want), []byte(code)) != 1 {
			continue
		}
		step := at.Unix() / totpPeriod
		a.totpMu.Lock()
		defer a.totpMu.Unlock()
		if step <= a.lastTOTPStep[userID] {
			return false
		}
		a.lastTOTPStep[userID] = step
		return true
	}
	return false
}
