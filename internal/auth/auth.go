package auth

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrUnauthorized     = errors.New("unauthorized: missing or invalid credentials")
	ErrForbiddenChannel = errors.New("forbidden: insufficient channel permissions")
)

// Authenticator handles API key verification for publishers and JWT claims for subscribers.
type Authenticator struct {
	publishAPIKey string
	jwtSecret     []byte
}

// NewAuthenticator creates a new auth controller.
func NewAuthenticator(apiKey, jwtSecret string) *Authenticator {
	return &Authenticator{
		publishAPIKey: apiKey,
		jwtSecret:     []byte(jwtSecret),
	}
}

// ValidatePublishRequest verifies the publisher's API key.
func (a *Authenticator) ValidatePublishRequest(r *http.Request) error {
	if a.publishAPIKey == "" {
		return nil // Authentication disabled
	}

	key := extractBearerOrQuery(r, "api_key", "X-API-Key")
	if key == "" {
		return ErrUnauthorized
	}

	if subtle.ConstantTimeCompare([]byte(key), []byte(a.publishAPIKey)) != 1 {
		return ErrUnauthorized
	}

	return nil
}

// SubscriberClaims represents expected JWT claims for SSE subscriptions.
type SubscriberClaims struct {
	jwt.RegisteredClaims
	Channels []string `json:"channels"`
}

// ValidateSubscribeRequest verifies JWT authentication and channel access permissions.
func (a *Authenticator) ValidateSubscribeRequest(r *http.Request, requestedChannels []string) error {
	if len(a.jwtSecret) == 0 {
		return nil // Subscriber auth disabled
	}

	tokenStr := extractBearerOrQuery(r, "token", "X-Auth-Token")
	if tokenStr == "" {
		return ErrUnauthorized
	}

	claims := &SubscriberClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrUnauthorized
		}
		return a.jwtSecret, nil
	})

	if err != nil || !token.Valid {
		return ErrUnauthorized
	}

	// Verify requested channels against authorized patterns in claims
	for _, reqCh := range requestedChannels {
		if !isChannelPermitted(reqCh, claims.Channels) {
			return ErrForbiddenChannel
		}
	}

	return nil
}

func isChannelPermitted(requested string, allowedPatterns []string) bool {
	for _, pattern := range allowedPatterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "*" || pattern == requested {
			return true
		}
		if strings.HasSuffix(pattern, ".*") {
			prefix := strings.TrimSuffix(pattern, ".*")
			if strings.HasPrefix(requested, prefix+".") {
				return true
			}
		}
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(requested, prefix) {
				return true
			}
		}
	}
	return false
}

func extractBearerOrQuery(r *http.Request, queryParam, customHeader string) string {
	// 1. Authorization header: Bearer <token>
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
	}

	// 2. Custom header
	if customHeader != "" {
		if val := r.Header.Get(customHeader); val != "" {
			return strings.TrimSpace(val)
		}
	}

	// 3. Query parameter
	if queryParam != "" {
		if val := r.URL.Query().Get(queryParam); val != "" {
			return strings.TrimSpace(val)
		}
	}

	return ""
}
