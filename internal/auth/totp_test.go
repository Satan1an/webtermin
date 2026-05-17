package auth

import (
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func TestValidateTOTP_AcceptsCurrentCode(t *testing.T) {
	key, err := GenerateTOTPSecret("webtermin-test", "alice")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	code, err := totp.GenerateCode(key.Secret(), time.Now())
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}
	if !ValidateTOTP(key.Secret(), code) {
		t.Fatal("freshly-generated TOTP code was rejected")
	}
}

func TestValidateTOTP_RejectsGarbage(t *testing.T) {
	key, _ := GenerateTOTPSecret("webtermin-test", "alice")
	for _, code := range []string{"", "abc", "000000", "12345"} {
		if ValidateTOTP(key.Secret(), code) {
			t.Fatalf("garbage code %q was accepted", code)
		}
	}
}

func TestValidateTOTP_RejectsEmptySecret(t *testing.T) {
	if ValidateTOTP("", "123456") {
		t.Fatal("validation succeeded with empty secret — must always fail")
	}
}
