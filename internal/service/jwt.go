package service

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// JwtService specifies service to generate and parse JWT tokens
type JwtService struct {
	tokenExp  int
	secretKey string
}

// NewJwtService creates new JwtService instance
func NewJwtService(tokenExp int, secretKey string) *JwtService {
	return &JwtService{
		tokenExp:  tokenExp,
		secretKey: secretKey,
	}
}

// GenerateJWT generates JWT for specified login
func (service *JwtService) GenerateJWT(login string) (string, error) {
	liveTime := time.Hour * time.Duration(service.tokenExp)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   login,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(liveTime)),
	})

	tokenString, err := token.SignedString([]byte(service.secretKey))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// GetSubjectFromJWT parses JWT and takes subj from it's structure
func (service *JwtService) GetSubjectFromJWT(jwtString string) (string, error) {
	claims := &jwt.RegisteredClaims{}

	token, err := jwt.ParseWithClaims(jwtString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(service.secretKey), nil
	})

	if err != nil {
		return "", err
	}

	if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok && token.Valid {
		return claims.Subject, nil
	}

	return "", errors.New("invalid token")
}
