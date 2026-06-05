package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jscyril/golang_music_player/api"
)

// TestRegisterAndAuthenticate verifies the full registration and login flow
// using bcrypt for password hashing and verification.
func TestRegisterAndAuthenticate(t *testing.T) {
	tmpDir := t.TempDir()
	usersFile := filepath.Join(tmpDir, "users.json")

	svc, err := NewService(usersFile)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	// Test registration
	user, err := svc.Register(api.RegisterRequest{
		Username: "testuser",
		Password: "securePassword123!",
		Role:     "user",
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if user.Username != "testuser" {
		t.Errorf("expected username 'testuser', got %q", user.Username)
	}
	if user.PasswordHash == "" {
		t.Error("password hash should not be empty after registration")
	}
	if user.PasswordHash == "securePassword123!" {
		t.Fatal("CRITICAL: password is stored in plain text, not hashed!")
	}

	// Test duplicate registration fails
	_, err = svc.Register(api.RegisterRequest{Username: "testuser", Password: "other"})
	if err != ErrUserAlreadyExists {
		t.Errorf("expected ErrUserAlreadyExists, got %v", err)
	}

	// Test successful authentication
	authedUser, err := svc.Authenticate(api.LoginRequest{
		Username: "testuser",
		Password: "securePassword123!",
	})
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
	if authedUser.Username != "testuser" {
		t.Errorf("expected authenticated user 'testuser', got %q", authedUser.Username)
	}

	// Test wrong password fails
	_, err = svc.Authenticate(api.LoginRequest{
		Username: "testuser",
		Password: "wrongPassword",
	})
	if err != ErrInvalidPassword {
		t.Errorf("expected ErrInvalidPassword, got %v", err)
	}

	// Test non-existent user fails
	_, err = svc.Authenticate(api.LoginRequest{
		Username: "nonexistent",
		Password: "anything",
	})
	if err != ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

// TestEmptyCredentials verifies that empty usernames/passwords are rejected
func TestEmptyCredentials(t *testing.T) {
	tmpDir := t.TempDir()
	svc, _ := NewService(filepath.Join(tmpDir, "users.json"))

	_, err := svc.Register(api.RegisterRequest{Username: "", Password: "pass"})
	if err != ErrEmptyCredentials {
		t.Errorf("expected ErrEmptyCredentials for empty username, got %v", err)
	}

	_, err = svc.Register(api.RegisterRequest{Username: "user", Password: ""})
	if err != ErrEmptyCredentials {
		t.Errorf("expected ErrEmptyCredentials for empty password, got %v", err)
	}
}

// TestPersistence verifies that user data survives service restarts
func TestPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	usersFile := filepath.Join(tmpDir, "users.json")

	// Create and register user
	svc1, _ := NewService(usersFile)
	svc1.Register(api.RegisterRequest{Username: "persistent", Password: "mypass", Role: "admin"})

	// Verify file was created
	if _, err := os.Stat(usersFile); os.IsNotExist(err) {
		t.Fatal("users.json was not created")
	}

	// Create new service instance (simulates restart)
	svc2, err := NewService(usersFile)
	if err != nil {
		t.Fatalf("Failed to reload service: %v", err)
	}

	// Authenticate with the persisted user
	user, err := svc2.Authenticate(api.LoginRequest{Username: "persistent", Password: "mypass"})
	if err != nil {
		t.Fatalf("Failed to authenticate after reload: %v", err)
	}
	if user.Role != "user" {
		t.Errorf("expected public registration to force role 'user', got %q", user.Role)
	}
}

// TestJWTRoundtrip verifies JWT generation and validation
func TestJWTRoundtrip(t *testing.T) {
	secret := []byte("test-secret-key")

	token, err := GenerateToken("alice", "admin", secret, 1*60*1e9) // 1 minute
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	claims, err := ValidateToken(token, secret)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if claims.Username != "alice" {
		t.Errorf("expected username 'alice', got %q", claims.Username)
	}
	if claims.Role != "admin" {
		t.Errorf("expected role 'admin', got %q", claims.Role)
	}

	// Test with wrong secret
	_, err = ValidateToken(token, []byte("wrong-secret"))
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken with wrong secret, got %v", err)
	}
}
