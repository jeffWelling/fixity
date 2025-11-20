package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/jeffanddom/fixity/internal/database"
)

// Service handles authentication operations
type Service struct {
	db              *database.Database
	sessionDuration time.Duration
	bcryptCost      int
}

// Config holds authentication configuration
type Config struct {
	SessionDuration time.Duration // How long sessions remain valid
	BcryptCost      int           // Bcrypt cost (4-31, default 10)
}

// NewService creates a new authentication service
func NewService(db *database.Database, config Config) *Service {
	// Set defaults
	if config.SessionDuration <= 0 {
		config.SessionDuration = 24 * time.Hour // Default: 24 hours
	}
	if config.BcryptCost <= 0 {
		config.BcryptCost = bcrypt.DefaultCost // Default: 10
	}

	return &Service{
		db:              db,
		sessionDuration: config.SessionDuration,
		bcryptCost:      config.BcryptCost,
	}
}

// HashPassword hashes a password using bcrypt
func (s *Service) HashPassword(password string) (string, error) {
	if len(password) == 0 {
		return "", fmt.Errorf("password cannot be empty")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.bcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	return string(hash), nil
}

// ComparePassword compares a password with a hash
func (s *Service) ComparePassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// Login authenticates a user and creates a session
func (s *Service) Login(ctx context.Context, username, password string) (*database.Session, error) {
	// Get user by username
	user, err := s.db.Users.GetByUsername(ctx, username)
	if err != nil || user == nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Compare password
	if err := s.ComparePassword(user.PasswordHash, password); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Generate session token
	token, err := s.generateToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate session token: %w", err)
	}

	// Create session
	session := &database.Session{
		Token:     token,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(s.sessionDuration),
	}

	if err := s.db.Sessions.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Update user's last login time
	now := time.Now()
	user.LastLogin = &now
	if err := s.db.Users.Update(ctx, user); err != nil {
		// Log error but don't fail login
		_ = err
	}

	return session, nil
}

// Logout invalidates a session
func (s *Service) Logout(ctx context.Context, token string) error {
	if err := s.db.Sessions.Delete(ctx, token); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

// ValidateSession checks if a session token is valid
func (s *Service) ValidateSession(ctx context.Context, token string) (*database.User, error) {
	// Get session
	session, err := s.db.Sessions.GetByToken(ctx, token)
	if err != nil || session == nil {
		return nil, fmt.Errorf("invalid session")
	}

	// Check if expired
	if time.Now().After(session.ExpiresAt) {
		// Delete expired session
		s.db.Sessions.Delete(ctx, token)
		return nil, fmt.Errorf("session expired")
	}

	// Get user
	user, err := s.db.Users.GetByID(ctx, session.UserID)
	if err != nil || user == nil {
		return nil, fmt.Errorf("user not found")
	}

	return user, nil
}

// CleanupExpiredSessions removes all expired sessions from the database
func (s *Service) CleanupExpiredSessions(ctx context.Context) (int64, error) {
	deleted, err := s.db.Sessions.DeleteExpired(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired sessions: %w", err)
	}

	return deleted, nil
}

// CreateUser creates a new user with hashed password
func (s *Service) CreateUser(ctx context.Context, username, password, email string, isAdmin bool) (*database.User, error) {
	// Hash password
	hash, err := s.HashPassword(password)
	if err != nil {
		return nil, err
	}

	// Create user
	var emailPtr *string
	if email != "" {
		emailPtr = &email
	}

	user := &database.User{
		Username:     username,
		PasswordHash: hash,
		Email:        emailPtr,
		IsAdmin:      isAdmin,
	}

	if err := s.db.Users.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// ChangePassword changes a user's password
func (s *Service) ChangePassword(ctx context.Context, userID int64, oldPassword, newPassword string) error {
	// Get user
	user, err := s.db.Users.GetByID(ctx, userID)
	if err != nil || user == nil {
		return fmt.Errorf("user not found")
	}

	// Verify old password
	if err := s.ComparePassword(user.PasswordHash, oldPassword); err != nil {
		return fmt.Errorf("invalid current password")
	}

	// Hash new password
	hash, err := s.HashPassword(newPassword)
	if err != nil {
		return err
	}

	// Update password
	if err := s.db.Users.UpdatePassword(ctx, userID, hash); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// generateToken generates a cryptographically secure random token
func (s *Service) generateToken() (string, error) {
	bytes := make([]byte, 32) // 256 bits
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(bytes), nil
}
