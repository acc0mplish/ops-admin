package model

import "time"

type IntegrationAIModel struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	Name           string    `json:"name" gorm:"size:128;not null;index"`
	Provider       string    `json:"provider" gorm:"size:32;not null;default:openai_compatible;index"`
	BaseURL        string    `json:"baseUrl" gorm:"size:1024;not null"`
	APIKey         string    `json:"-" gorm:"type:text"`
	Model          string    `json:"model" gorm:"size:128;not null"`
	SystemPrompt   string    `json:"systemPrompt" gorm:"type:text"`
	Temperature    float64   `json:"temperature" gorm:"default:0.2"`
	MaxTokens      int       `json:"maxTokens" gorm:"default:2048"`
	TimeoutSeconds int       `json:"timeoutSeconds" gorm:"default:60"`
	IsDefault      bool      `json:"isDefault" gorm:"default:false;index"`
	Status         int       `json:"status" gorm:"default:1;index"`
	Description    string    `json:"description" gorm:"size:500"`
	CreatedAt      time.Time `json:"createTime"`
	UpdatedAt      time.Time `json:"updateTime"`
}

func (IntegrationAIModel) TableName() string { return "integration_ai_model" }

type IntegrationAIConversation struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	UserID        uint       `json:"userId" gorm:"not null;index"`
	Username      string     `json:"username" gorm:"size:128;index"`
	ModelID       uint       `json:"modelId" gorm:"index"`
	Title         string     `json:"title" gorm:"size:255;not null;index"`
	Status        int        `json:"status" gorm:"default:1;index"`
	Pinned        bool       `json:"pinned" gorm:"default:false;index"`
	MessageCount  int        `json:"messageCount" gorm:"default:0"`
	LastMessageAt *time.Time `json:"lastMessageAt" gorm:"index"`
	CreatedAt     time.Time  `json:"createTime"`
	UpdatedAt     time.Time  `json:"updateTime"`
}

func (IntegrationAIConversation) TableName() string { return "integration_ai_conversation" }

type IntegrationAIMessage struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	ConversationID uint      `json:"conversationId" gorm:"not null;index"`
	Role           string    `json:"role" gorm:"size:32;not null;index"`
	Content        string    `json:"content" gorm:"type:longtext"`
	ToolName       string    `json:"toolName" gorm:"size:128;index"`
	ToolCallID     string    `json:"toolCallId" gorm:"size:128;index"`
	ToolPayload    string    `json:"toolPayload" gorm:"type:longtext"`
	Status         string    `json:"status" gorm:"size:32;default:completed;index"`
	CreatedAt      time.Time `json:"createTime" gorm:"index"`
}

func (IntegrationAIMessage) TableName() string { return "integration_ai_message" }

type IntegrationAIToolConfig struct {
	ID                  uint      `json:"id" gorm:"primaryKey"`
	ToolKey             string    `json:"toolKey" gorm:"size:128;not null;uniqueIndex"`
	Enabled             bool      `json:"enabled" gorm:"default:true;index"`
	RequireConfirmation bool      `json:"requireConfirmation" gorm:"default:false"`
	UpdatedAt           time.Time `json:"updateTime"`
	CreatedAt           time.Time `json:"createTime"`
}

func (IntegrationAIToolConfig) TableName() string { return "integration_ai_tool_config" }

type IntegrationAIToolAction struct {
	ID             uint       `json:"id" gorm:"primaryKey"`
	ConversationID uint       `json:"conversationId" gorm:"not null;index"`
	MessageID      uint       `json:"messageId" gorm:"index"`
	UserID         uint       `json:"userId" gorm:"not null;index"`
	ToolKey        string     `json:"toolKey" gorm:"size:128;not null;index"`
	ArgumentsJSON  string     `json:"argumentsJson" gorm:"type:longtext"`
	Status         string     `json:"status" gorm:"size:32;default:pending;index"`
	ResultJSON     string     `json:"resultJson" gorm:"type:longtext"`
	ConfirmedBy    string     `json:"confirmedBy" gorm:"size:128"`
	ConfirmedAt    *time.Time `json:"confirmedAt"`
	CreatedAt      time.Time  `json:"createTime"`
	UpdatedAt      time.Time  `json:"updateTime"`
}

func (IntegrationAIToolAction) TableName() string { return "integration_ai_tool_action" }

// IntegrationAIKnowledgeDocument stores Markdown content locally for the AI assistant.
type IntegrationAIKnowledgeDocument struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	Name       string    `json:"name" gorm:"size:255;not null;index"`
	FileName   string    `json:"fileName" gorm:"size:255;not null"`
	SourceType string    `json:"sourceType" gorm:"size:32;not null;default:manual;index"`
	Content    string    `json:"content" gorm:"type:longtext;not null"`
	Status     int       `json:"status" gorm:"default:1;index"`
	CreatedAt  time.Time `json:"createTime"`
	UpdatedAt  time.Time `json:"updateTime"`
}

func (IntegrationAIKnowledgeDocument) TableName() string { return "integration_ai_knowledge_document" }
