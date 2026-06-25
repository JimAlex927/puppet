package agent

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"puppet/internal/model"
	"puppet/internal/secret"
	"time"

	"puppet/internal/auth"

	"gorm.io/gorm"
)

type CreateRequest struct {
	Name        string   `json:"name"`
	EndpointURL string   `json:"endpointUrl"`
	Labels      []string `json:"labels"`
}

type UpdateRequest struct {
	Name        string   `json:"name"`
	EndpointURL string   `json:"endpointUrl"`
	Labels      []string `json:"labels"`
	Status      string   `json:"status"`
}

type CreateResponse struct {
	Agent model.Agent `json:"agent"`
	Token string      `json:"token"`
}

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) List() ([]model.Agent, error) {
	var agents []model.Agent
	err := s.db.Order("id asc").Find(&agents).Error
	return agents, err
}

func (s *Service) Get(id uint) (model.Agent, error) {
	var agent model.Agent
	err := s.db.First(&agent, id).Error
	return agent, err
}

func (s *Service) Create(req CreateRequest) (CreateResponse, error) {
	if req.Name == "" || req.EndpointURL == "" {
		return CreateResponse{}, fmt.Errorf("name and endpointUrl are required")
	}
	token, err := randomToken()
	if err != nil {
		return CreateResponse{}, err
	}
	encrypted, err := secret.EncryptText(token)
	if err != nil {
		return CreateResponse{}, err
	}
	labels, _ := json.Marshal(req.Labels)
	agent := model.Agent{
		Name:        req.Name,
		EndpointURL: req.EndpointURL,
		LabelsJSON:  string(labels),
		TokenHash:   auth.HashToken(token),
		TokenSecret: encrypted,
		Status:      "offline",
	}
	err = s.db.Create(&agent).Error
	return CreateResponse{Agent: agent, Token: token}, err
}

func (s *Service) Update(id uint, req UpdateRequest) (model.Agent, error) {
	agent, err := s.Get(id)
	if err != nil {
		return agent, err
	}
	if req.Name != "" {
		agent.Name = req.Name
	}
	agent.EndpointURL = req.EndpointURL
	if req.Labels != nil {
		labels, _ := json.Marshal(req.Labels)
		agent.LabelsJSON = string(labels)
	}
	if req.Status != "" {
		agent.Status = req.Status
	}
	err = s.db.Save(&agent).Error
	return agent, err
}

func (s *Service) Delete(id uint) error {
	return s.db.Delete(&model.Agent{}, id).Error
}

func (s *Service) Token(agent model.Agent) (string, error) {
	return secret.DecryptText(agent.TokenSecret)
}

func (s *Service) AuthenticateBearer(token string) (model.Agent, error) {
	var agent model.Agent
	err := s.db.Where("token_hash = ?", auth.HashToken(token)).First(&agent).Error
	return agent, err
}

func (s *Service) Heartbeat(agent model.Agent, osName string, arch string, hostname string) (model.Agent, error) {
	now := time.Now()
	agent.OS = osName
	agent.Arch = arch
	agent.Hostname = hostname
	agent.Status = "online"
	agent.LastHeartbeatAt = &now
	err := s.db.Save(&agent).Error
	return agent, err
}

func randomToken() (string, error) {
	content := make([]byte, 32)
	if _, err := rand.Read(content); err != nil {
		return "", err
	}
	return hex.EncodeToString(content), nil
}
