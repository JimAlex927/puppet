package credential

import (
	"encoding/json"
	"fmt"

	"puppet/internal/model"
	"puppet/internal/node"
	"puppet/internal/secret"

	"gorm.io/gorm"
)

const (
	TypeUsernamePassword = "username_password"
	TypeToken            = "token"
	TypeSSHKey           = "ssh_key"
)

type PublicCredential struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Username    string `json:"username"`
	HasSecret   bool   `json:"hasSecret"`
	CreatedAt   any    `json:"createdAt"`
	UpdatedAt   any    `json:"updatedAt"`
}

type UpsertRequest struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	Token       string `json:"token"`
	PrivateKey  string `json:"privateKey"`
}

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) List() ([]PublicCredential, error) {
	var credentials []model.Credential
	if err := s.db.Order("id desc").Find(&credentials).Error; err != nil {
		return nil, err
	}
	result := make([]PublicCredential, 0, len(credentials))
	for _, item := range credentials {
		result = append(result, toPublic(item))
	}
	return result, nil
}

func (s *Service) Get(id uint) (PublicCredential, error) {
	var credential model.Credential
	err := s.db.First(&credential, id).Error
	return toPublic(credential), err
}

func (s *Service) Create(req UpsertRequest) (PublicCredential, error) {
	if err := validate(req, true); err != nil {
		return PublicCredential{}, err
	}
	secretJSON, err := encodeSecrets(req)
	if err != nil {
		return PublicCredential{}, err
	}
	credential := model.Credential{
		Name:        req.Name,
		Type:        req.Type,
		Description: req.Description,
		Username:    req.Username,
		SecretJSON:  secretJSON,
	}
	err = s.db.Create(&credential).Error
	return toPublic(credential), err
}

func (s *Service) Update(id uint, req UpsertRequest) (PublicCredential, error) {
	var credential model.Credential
	if err := s.db.First(&credential, id).Error; err != nil {
		return PublicCredential{}, err
	}
	if req.Type == "" {
		req.Type = credential.Type
	}
	if err := validate(req, false); err != nil {
		return PublicCredential{}, err
	}
	credential.Name = req.Name
	credential.Type = req.Type
	credential.Description = req.Description
	credential.Username = req.Username
	if hasSecretInput(req) {
		secretJSON, err := encodeSecrets(req)
		if err != nil {
			return PublicCredential{}, err
		}
		credential.SecretJSON = secretJSON
	}
	err := s.db.Save(&credential).Error
	return toPublic(credential), err
}

func (s *Service) Delete(id uint) error {
	return s.db.Delete(&model.Credential{}, id).Error
}

func (s *Service) Resolve(id uint) (*node.Credential, error) {
	if id == 0 {
		return nil, nil
	}
	var credential model.Credential
	if err := s.db.First(&credential, id).Error; err != nil {
		return nil, err
	}
	secrets, err := decodeSecrets(credential.SecretJSON)
	if err != nil {
		return nil, err
	}
	return &node.Credential{
		ID:          credential.ID,
		Name:        credential.Name,
		Type:        credential.Type,
		Description: credential.Description,
		Username:    credential.Username,
		Secrets:     secrets,
	}, nil
}

func validate(req UpsertRequest, requireSecret bool) error {
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	switch req.Type {
	case TypeUsernamePassword:
		if req.Username == "" {
			return fmt.Errorf("username is required")
		}
		if requireSecret && req.Password == "" {
			return fmt.Errorf("password is required")
		}
	case TypeToken:
		if requireSecret && req.Token == "" {
			return fmt.Errorf("token is required")
		}
	case TypeSSHKey:
		if req.Username == "" {
			req.Username = "git"
		}
		if requireSecret && req.PrivateKey == "" {
			return fmt.Errorf("privateKey is required")
		}
	default:
		return fmt.Errorf("unsupported credential type %q", req.Type)
	}
	return nil
}

func hasSecretInput(req UpsertRequest) bool {
	return req.Password != "" || req.Token != "" || req.PrivateKey != ""
}

func encodeSecrets(req UpsertRequest) (string, error) {
	secrets := map[string]string{}
	if req.Password != "" {
		secrets["password"] = req.Password
	}
	if req.Token != "" {
		secrets["token"] = req.Token
	}
	if req.PrivateKey != "" {
		secrets["privateKey"] = req.PrivateKey
	}
	content, err := json.Marshal(secrets)
	if err != nil {
		return "", err
	}
	return encrypt(content)
}

func decodeSecrets(value string) (map[string]string, error) {
	if value == "" {
		return map[string]string{}, nil
	}
	content, err := decrypt(value)
	if err != nil {
		return nil, err
	}
	secrets := map[string]string{}
	if err := json.Unmarshal(content, &secrets); err != nil {
		return nil, err
	}
	return secrets, nil
}

func toPublic(credential model.Credential) PublicCredential {
	return PublicCredential{
		ID:          credential.ID,
		Name:        credential.Name,
		Type:        credential.Type,
		Description: credential.Description,
		Username:    credential.Username,
		HasSecret:   credential.SecretJSON != "",
		CreatedAt:   credential.CreatedAt,
		UpdatedAt:   credential.UpdatedAt,
	}
}

func encrypt(plaintext []byte) (string, error) {
	return secret.EncryptText(string(plaintext))
}

func decrypt(value string) ([]byte, error) {
	plain, err := secret.DecryptText(value)
	return []byte(plain), err
}
