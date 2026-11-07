package auth

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestAuth_PublishKey(t *testing.T) {
	auth := NewAuthenticator("my-secret-key", "")

	// 1. Missing key
	req := httptest.NewRequest("POST", "/publish", nil)
	if err := auth.ValidatePublishRequest(req); err != ErrUnauthorized {
		t.Fatalf("expected ErrUnauthorized for missing key, got %v", err)
	}

	// 2. Wrong key
	req = httptest.NewRequest("POST", "/publish", nil)
	req.Header.Set("Authorization", "Bearer invalid-key")
	if err := auth.ValidatePublishRequest(req); err != ErrUnauthorized {
		t.Fatalf("expected ErrUnauthorized for invalid key, got %v", err)
	}

	// 3. Valid key via Header
	req = httptest.NewRequest("POST", "/publish", nil)
	req.Header.Set("Authorization", "Bearer my-secret-key")
	if err := auth.ValidatePublishRequest(req); err != nil {
		t.Fatalf("expected nil for valid key, got %v", err)
	}

	// 4. Valid key via Query Param
	req = httptest.NewRequest("POST", "/publish?api_key=my-secret-key", nil)
	if err := auth.ValidatePublishRequest(req); err != nil {
		t.Fatalf("expected nil for valid query key, got %v", err)
	}
}

func TestAuth_SubscribeJWT(t *testing.T) {
	secret := "jwt-secret-xyz"
	auth := NewAuthenticator("", secret)

	// Generate valid token for ["public.*", "user-42"]
	claims := SubscriberClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Channels: []string{"public.*", "user-42"},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	// 1. Authorized subscription: "public.chat" and "user-42"
	req := httptest.NewRequest("GET", "/subscribe?token="+signedToken, nil)
	if err := auth.ValidateSubscribeRequest(req, []string{"public.chat", "user-42"}); err != nil {
		t.Fatalf("expected access permitted, got %v", err)
	}

	// 2. Unauthorized subscription: "secret-vault"
	if err := auth.ValidateSubscribeRequest(req, []string{"secret-vault"}); err != ErrForbiddenChannel {
		t.Fatalf("expected ErrForbiddenChannel, got %v", err)
	}

	// 3. Expired or invalid token
	reqInvalid := httptest.NewRequest("GET", "/subscribe?token=bad-token", nil)
	if err := auth.ValidateSubscribeRequest(reqInvalid, []string{"public.chat"}); err != ErrUnauthorized {
		t.Fatalf("expected ErrUnauthorized for bad token, got %v", err)
	}
}
