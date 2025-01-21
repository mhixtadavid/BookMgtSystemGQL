package auth

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	// jwtSecret = []byte("tojumiappsecret")
	jwtSecret       = []byte(os.Getenv("SECRET"))
	ErrInvalidToken = errors.New("invalid token")
)

// Claims with role
type Claims struct {
	UserID   string `json:"userId" bson:"userId"`
	UserRole string `json:"userRole" bson:"userRole"`
	jwt.RegisteredClaims
}

// GenerateJWT creates a new token for a user with a specific role
func GenerateJWT(userID, userRole string) (string, error) {
	expirationTime := time.Now().Add(time.Hour * 24 * 365 * 5)
	claims := &Claims{
		UserID:   userID,
		UserRole: userRole,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// ValidateJWT validates the token and returns the claims if valid
func ValidateJWT(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenStr,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {
			// Validate the signing method
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, ErrInvalidToken
			}
			return jwtSecret, nil
		},
	)

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, jwt.ErrTokenExpired
		}
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
