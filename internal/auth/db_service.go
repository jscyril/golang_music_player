package auth

import (
	"context"
	"fmt"
	"log"

	"github.com/jscyril/golang_music_player/api"
	"github.com/jscyril/golang_music_player/internal/database"
	"golang.org/x/crypto/bcrypt"
)

// DBService handles user authentication backed by PostgreSQL.
// Uses the UserRepo for persistence and bcrypt for credential hashing.
type DBService struct {
	repo *database.UserRepo
}

// NewDBService creates a new database-backed auth service.
func NewDBService(repo *database.UserRepo) *DBService {
	return &DBService{repo: repo}
}

// Register creates a new user with a bcrypt-hashed password, storing in PostgreSQL.
func (s *DBService) Register(ctx context.Context, req api.RegisterRequest) (*api.User, error) {
	if req.Username == "" || req.Password == "" {
		return nil, ErrEmptyCredentials
	}

	// Hash the password using bcrypt (cost 12 for robust security)
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to hash password: %w", err)
	}

	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("auth: failed to generate user id: %w", err)
	}

	user, err := s.repo.Create(ctx, id, req.Username, string(hash), "user")
	if err != nil {
		// Map DB duplicate key to our sentinel error
		if err.Error() == "username already taken" {
			return nil, ErrUserAlreadyExists
		}
		return nil, fmt.Errorf("auth: registration failed: %w", err)
	}

	log.Printf("[AUTH-DB] User registered: %s (role: %s)", user.Username, user.Role)
	return user, nil
}

// Authenticate verifies credentials against PostgreSQL using bcrypt comparison.
func (s *DBService) Authenticate(ctx context.Context, req api.LoginRequest) (*api.User, error) {
	if req.Username == "" || req.Password == "" {
		return nil, ErrEmptyCredentials
	}

	user, err := s.repo.GetByUsername(ctx, req.Username)
	if err != nil {
		return nil, ErrUserNotFound
	}

	// bcrypt.CompareHashAndPassword handles salt extraction automatically
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidPassword
	}

	return user, nil
}

// GetUser retrieves a user by username from PostgreSQL.
func (s *DBService) GetUser(ctx context.Context, username string) (*api.User, error) {
	user, err := s.repo.GetByUsername(ctx, username)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return user, nil
}
