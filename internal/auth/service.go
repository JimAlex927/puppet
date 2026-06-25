package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"puppet/internal/model"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string     `json:"token"`
	User  model.User `json:"user"`
}

type UserRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	Password    string `json:"password"`
}

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Login(req LoginRequest) (LoginResponse, error) {
	var user model.User
	if err := s.db.Where("username = ?", req.Username).First(&user).Error; err != nil {
		return LoginResponse{}, fmt.Errorf("invalid username or password")
	}
	if user.Status != "active" {
		return LoginResponse{}, fmt.Errorf("user is disabled")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return LoginResponse{}, fmt.Errorf("invalid username or password")
	}

	// Clean up this user's expired sessions before creating a new one.
	_ = s.db.Where("user_id = ? AND expires_at <= ?", user.ID, time.Now()).Delete(&model.Session{}).Error

	token, err := randomToken()
	if err != nil {
		return LoginResponse{}, err
	}
	now := time.Now()
	session := model.Session{
		UserID:    user.ID,
		TokenHash: HashToken(token),
		ExpiresAt: now.Add(24 * time.Hour),
	}
	if err := s.db.Create(&session).Error; err != nil {
		return LoginResponse{}, err
	}
	user.LastLoginAt = &now
	_ = s.db.Save(&user).Error
	return LoginResponse{Token: token, User: user}, nil
}

func (s *Service) Authenticate(token string) (model.User, error) {
	var session model.Session
	if err := s.db.Where("token_hash = ? AND expires_at > ?", HashToken(token), time.Now()).First(&session).Error; err != nil {
		return model.User{}, err
	}
	var user model.User
	if err := s.db.First(&user, session.UserID).Error; err != nil {
		return model.User{}, err
	}
	if user.Status != "active" {
		return model.User{}, fmt.Errorf("user is disabled")
	}
	return user, nil
}

func (s *Service) Logout(token string) error {
	return s.db.Where("token_hash = ?", HashToken(token)).Delete(&model.Session{}).Error
}

func (s *Service) ListUsers() ([]model.User, error) {
	var users []model.User
	err := s.db.Order("id asc").Find(&users).Error
	return users, err
}

func (s *Service) CreateUser(req UserRequest) (model.User, error) {
	if req.Username == "" || req.Password == "" {
		return model.User{}, fmt.Errorf("username and password are required")
	}
	if req.Role == "" {
		req.Role = "operator"
	}
	if req.Status == "" {
		req.Status = "active"
	}
	hash, err := HashPassword(req.Password)
	if err != nil {
		return model.User{}, err
	}
	user := model.User{
		Username:     req.Username,
		DisplayName:  req.DisplayName,
		Role:         req.Role,
		Status:       req.Status,
		PasswordHash: hash,
	}
	err = s.db.Create(&user).Error
	return user, err
}

func (s *Service) UpdateUser(id uint, req UserRequest) (model.User, error) {
	var user model.User
	if err := s.db.First(&user, id).Error; err != nil {
		return user, err
	}
	if req.Username != "" {
		user.Username = req.Username
	}
	user.DisplayName = req.DisplayName
	if req.Role != "" {
		user.Role = req.Role
	}
	if req.Status != "" {
		user.Status = req.Status
	}
	if req.Password != "" {
		hash, err := HashPassword(req.Password)
		if err != nil {
			return user, err
		}
		user.PasswordHash = hash
	}
	err := s.db.Save(&user).Error
	return user, err
}

func (s *Service) DeleteUser(id uint) error {
	var count int64
	if err := s.db.Model(&model.User{}).Count(&count).Error; err != nil {
		return err
	}
	if count <= 1 {
		return fmt.Errorf("cannot delete the last user")
	}
	return s.db.Delete(&model.User{}, id).Error
}

func HashPassword(password string) (string, error) {
	content, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(content), err
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomToken() (string, error) {
	content := make([]byte, 32)
	if _, err := rand.Read(content); err != nil {
		return "", err
	}
	return hex.EncodeToString(content), nil
}
