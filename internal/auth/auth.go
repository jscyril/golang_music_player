package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/jscyril/golang_music_player/api"
	"golang.org/x/crypto/bcrypt"
)

// Sentinel errors for the auth package
var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("username already taken")
	ErrInvalidPassword   = errors.New("invalid password")
	ErrEmptyCredentials  = errors.New("username and password cannot be empty")
)

// Service handles user registration, authentication, and persistence.
// It uses bcrypt for robust credential hashing (security requirement).
type Service struct {
	mu       sync.RWMutex
	users    map[string]*api.User // keyed by username for O(1) lookup
	filePath string
}

// userRecord is the internal representation stored to disk, including the hash
type userRecord struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

// NewService creates a new auth service, loading any existing users from disk.
func NewService(filePath string) (*Service, error) {
	s := &Service{
		users:    make(map[string]*api.User),
		filePath: filePath,
	}
	if err := s.loadUsers(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("auth: failed to load users: %w", err)
	}
	return s, nil
}

// Register creates a new user with a bcrypt-hashed password.
// Demonstrates: bcrypt security, mutex concurrency safety, error handling.
func (s *Service) Register(req api.RegisterRequest) (*api.User, error) {
	if req.Username == "" || req.Password == "" {
		return nil, ErrEmptyCredentials
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate usernames
	if _, exists := s.users[req.Username]; exists {
		return nil, ErrUserAlreadyExists
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

	user := &api.User{
		ID:           id,
		Username:     req.Username,
		PasswordHash: string(hash),
		Role:         "user",
		CreatedAt:    time.Now(),
	}

	s.users[req.Username] = user

	if err := s.saveUsers(); err != nil {
		return nil, fmt.Errorf("auth: failed to persist user: %w", err)
	}

	return user, nil
}

// Authenticate verifies a user's credentials using bcrypt comparison.
// Returns the user on success or an error on failure.
func (s *Service) Authenticate(req api.LoginRequest) (*api.User, error) {
	if req.Username == "" || req.Password == "" {
		return nil, ErrEmptyCredentials
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[req.Username]
	if !exists {
		return nil, ErrUserNotFound
	}

	// bcrypt.CompareHashAndPassword automatically handles salt extraction
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidPassword
	}

	return user, nil
}

// GetUser retrieves a user by username (thread-safe).
func (s *Service) GetUser(username string) (*api.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[username]
	if !exists {
		return nil, ErrUserNotFound
	}
	return user, nil
}

// --- Persistence (JSON file-based) ---

func (s *Service) loadUsers() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	var records []userRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return fmt.Errorf("auth: failed to unmarshal users: %w", err)
	}

	for _, r := range records {
		s.users[r.Username] = &api.User{
			ID:           r.ID,
			Username:     r.Username,
			PasswordHash: r.PasswordHash,
			Role:         r.Role,
			CreatedAt:    r.CreatedAt,
		}
	}
	return nil
}

func (s *Service) saveUsers() error {
	records := make([]userRecord, 0, len(s.users))
	for _, u := range s.users {
		records = append(records, userRecord{
			ID:           u.ID,
			Username:     u.Username,
			PasswordHash: u.PasswordHash,
			Role:         u.Role,
			CreatedAt:    u.CreatedAt,
		})
	}

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("auth: failed to marshal users: %w", err)
	}
	return os.WriteFile(s.filePath, data, 0600)
}

// generateID creates a cryptographically random hex ID
func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
