package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	secretOnce sync.Once
	jwtSecret  string
)

func getSessionSecret() string {
	secretOnce.Do(func() {
		if s := os.Getenv("SESSION_SECRET"); s != "" {
			jwtSecret = s
			return
		}
		b := make([]byte, 32)
		_, _ = rand.Read(b)
		jwtSecret = hex.EncodeToString(b)
	})
	return jwtSecret
}

// ResetSessionSecretForTest allows unit tests to set a deterministic secret.
func setSessionSecret(s string) {
	secretOnce.Do(func() {})
	jwtSecret = s
}

// HashPassword produces "saltHex:hashHex" compatible with Node.js auth.js
// (crypto.scryptSync(password, saltHex, 64) with default N=16384, r=8, p=1).
func HashPassword(password string) string {
	saltBytes := make([]byte, 16)
	_, _ = rand.Read(saltBytes)
	salt := hex.EncodeToString(saltBytes)
	derived, _ := scryptKey([]byte(password), []byte(salt), 16384, 8, 1, 64)
	return salt + ":" + hex.EncodeToString(derived)
}

func VerifyPassword(password, stored string) bool {
	if stored == "" || !strings.Contains(stored, ":") {
		return false
	}
	parts := strings.SplitN(stored, ":", 2)
	salt, keyHex := parts[0], parts[1]
	keyBuf, err := hex.DecodeString(keyHex)
	if err != nil {
		return false
	}
	derived, err := scryptKey([]byte(password), []byte(salt), 16384, 8, 1, 64)
	if err != nil || len(derived) != len(keyBuf) {
		return false
	}
	return subtle.ConstantTimeCompare(derived, keyBuf) == 1
}

type TokenClaims struct {
	Role     string `json:"role"`
	ParentID string `json:"parentId,omitempty"`
	ChildID  string `json:"childId,omitempty"`
	FamilyID string `json:"familyId,omitempty"`
	AdminID  string `json:"adminId,omitempty"`
	Exp      int64  `json:"exp"`
}

func SignToken(claims TokenClaims, ttlSeconds int64) string {
	if ttlSeconds <= 0 {
		ttlSeconds = 60 * 60 * 24 * 14
	}
	claims.Exp = time.Now().Unix() + ttlSeconds
	raw, _ := json.Marshal(claims)
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, []byte(getSessionSecret()))
	mac.Write([]byte(encoded))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return encoded + "." + sig
}

func VerifyToken(token string) *TokenClaims {
	if token == "" {
		return nil
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil
	}
	encoded, sig := parts[0], parts[1]
	mac := hmac.New(sha256.New, []byte(getSessionSecret()))
	mac.Write([]byte(encoded))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(sig), []byte(expected)) != 1 {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil
	}
	var claims TokenClaims
	if err := json.Unmarshal(raw, &claims); err != nil {
		return nil
	}
	if claims.Exp == 0 || claims.Exp < time.Now().Unix() {
		return nil
	}
	return &claims
}

func GenID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func GenFamilyCode() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	out := make([]byte, 6)
	for i := 0; i < 6; i++ {
		out[i] = chars[int(b[i])%len(chars)]
	}
	return string(out)
}
