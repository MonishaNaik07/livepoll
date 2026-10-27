package services

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/your-username/live-polling-app/backend/internal/apperr"
	"github.com/your-username/live-polling-app/backend/internal/models"
	"github.com/your-username/live-polling-app/backend/internal/repositories"
	"github.com/your-username/live-polling-app/backend/internal/validation"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	users  *repositories.UserRepo
	secret []byte
	ttl    time.Duration
}

func NewAuthService(u *repositories.UserRepo, secret string, ttl time.Duration) *AuthService {
	return &AuthService{users: u, secret: []byte(secret), ttl: ttl}
}

// dummyHash equalises timing when the email doesn't exist (avoids user enumeration via timing).
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password"), bcrypt.DefaultCost)

func (s *AuthService) Signup(ctx context.Context, in validation.SignupInput) (*models.User, string, error) {
	if e := in.Validate(); e != nil {
		return nil, "", e
	}
	h, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", err
	}
	u := &models.User{Name: in.Name, Email: in.Email, PasswordHash: string(h)}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, "", err
	}
	tok, err := s.issue(u.ID)
	return u, tok, err
}

func (s *AuthService) Login(ctx context.Context, in validation.LoginInput) (*models.User, string, error) {
	if e := in.Validate(); e != nil {
		return nil, "", e
	}
	u, err := s.users.FindByEmail(ctx, in.Email)
	if err != nil {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(in.Password))
		if err == apperr.ErrNotFound {
			return nil, "", apperr.ErrBadCreds
		}
		return nil, "", err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.Password)) != nil {
		return nil, "", apperr.ErrBadCreds
	}
	tok, err := s.issue(u.ID)
	return u, tok, err
}

func (s *AuthService) Me(ctx context.Context, id primitive.ObjectID) (*models.User, error) {
	return s.users.FindByID(ctx, id)
}

func (s *AuthService) issue(id primitive.ObjectID) (string, error) {
	now := time.Now()
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject: id.Hex(), IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
	}).SignedString(s.secret)
}

// Parse verifies signature (HS256 only) and expiry, returning the user id.
func (s *AuthService) Parse(token string) (primitive.ObjectID, error) {
	var c jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(token, &c, func(*jwt.Token) (interface{}, error) { return s.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil {
		return primitive.NilObjectID, apperr.ErrUnauthorized
	}
	id, err := primitive.ObjectIDFromHex(c.Subject)
	if err != nil {
		return primitive.NilObjectID, apperr.ErrUnauthorized
	}
	return id, nil
}

func (s *AuthService) TTL() time.Duration { return s.ttl }
