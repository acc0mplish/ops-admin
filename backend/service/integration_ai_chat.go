package service

import (
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strings"
	"time"

	"ops-admin/backend/model"

	"gorm.io/gorm"
)

func (s *Service) ListIntegrationAIKnowledgeDocuments(keyword string) ([]map[string]any, error) {
	var documents []model.IntegrationAIKnowledgeDocument
	query := s.db.Order("updated_at DESC, id DESC")
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR file_name LIKE ?", like, like)
	}
	if err := query.Find(&documents).Error; err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(documents))
	for _, item := range documents {
		result = append(result, map[string]any{
			"id": item.ID, "name": item.Name, "fileName": item.FileName, "sourceType": item.SourceType,
			"content": item.Content, "status": item.Status, "createTime": item.CreatedAt, "updateTime": item.UpdatedAt,
		})
	}
	return result, nil
}

func (s *Service) SaveIntegrationAIKnowledgeDocument(payload IntegrationAIKnowledgeDocumentPayload) (map[string]any, error) {
	payload.Name = strings.TrimSpace(payload.Name)
	payload.Content = strings.TrimSpace(payload.Content)
	if payload.Name == "" {
		return nil, errors.New("knowledge-base document name is required")
	}
	if payload.Content == "" {
		return nil, errors.New("Markdown content is required")
	}
	if len([]rune(payload.Content)) > 500000 {
		return nil, errors.New("Markdown content must not exceed 500000 characters")
	}
	if payload.FileName == "" {
		payload.FileName = payload.Name + ".md"
	}
	if !strings.EqualFold(filepathExt(payload.FileName), ".md") {
		payload.FileName += ".md"
	}
	if payload.SourceType == "" {
		payload.SourceType = "manual"
	}
	if payload.Status != 2 {
		payload.Status = 1
	}
	item := model.IntegrationAIKnowledgeDocument{ID: payload.ID}
	if payload.ID > 0 {
		if err := s.db.First(&item, payload.ID).Error; err != nil {
			return nil, errors.New("knowledge-base document does not exist")
		}
	}
	item.Name, item.FileName, item.Content, item.Status, item.SourceType = payload.Name, payload.FileName, payload.Content, payload.Status, payload.SourceType
	var err error
	if item.ID == 0 {
		err = s.db.Create(&item).Error
	} else {
		err = s.db.Save(&item).Error
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": item.ID, "name": item.Name, "fileName": item.FileName, "sourceType": item.SourceType, "status": item.Status, "updateTime": item.UpdatedAt}, nil
}

func (s *Service) DeleteIntegrationAIKnowledgeDocument(id uint) error {
	if id == 0 {
		return errors.New("knowledge-base document ID is required")
	}
	return s.db.Delete(&model.IntegrationAIKnowledgeDocument{}, id).Error
}

func filepathExt(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx < 0 {
		return ""
	}
	return name[idx:]
}

func (s *Service) ListIntegrationAIModels() ([]map[string]any, error) {
	var list []model.IntegrationAIModel
	if err := s.db.Order("is_default DESC, id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(list))
	for _, item := range list {
		result = append(result, aiModelView(item))
	}
	return result, nil
}

func aiModelView(item model.IntegrationAIModel) map[string]any {
	return map[string]any{"id": item.ID, "name": item.Name, "provider": item.Provider, "baseUrl": item.BaseURL,
		"model": item.Model, "systemPrompt": item.SystemPrompt, "temperature": item.Temperature, "maxTokens": item.MaxTokens,
		"timeoutSeconds": item.TimeoutSeconds, "isDefault": item.IsDefault, "status": item.Status, "description": item.Description,
		"hasApiKey": strings.TrimSpace(item.APIKey) != "", "apiKeyMasked": maskSecret(item.APIKey), "createTime": item.CreatedAt, "updateTime": item.UpdatedAt}
}

func maskSecret(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return "********"
	}
	return value[:3] + strings.Repeat("*", 8) + value[len(value)-4:]
}

func (s *Service) SaveIntegrationAIModel(payload IntegrationAIModelPayload) (map[string]any, error) {
	payload.Name = strings.TrimSpace(payload.Name)
	payload.BaseURL = strings.TrimRight(strings.TrimSpace(payload.BaseURL), "/")
	payload.Model = strings.TrimSpace(payload.Model)
	if payload.Name == "" || payload.BaseURL == "" || payload.Model == "" {
		return nil, errors.New("model name, API URL, and model identifier are required")
	}
	if parsed, err := url.Parse(payload.BaseURL); err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("enter a valid OpenAI-compatible API URL")
	}
	if payload.Provider == "" {
		payload.Provider = "openai_compatible"
	}
	if payload.Status == 0 {
		payload.Status = 1
	}
	if payload.TimeoutSeconds < 5 {
		payload.TimeoutSeconds = 60
	}
	if payload.TimeoutSeconds > 600 {
		payload.TimeoutSeconds = 600
	}
	if payload.MaxTokens <= 0 {
		payload.MaxTokens = 2048
	}
	if payload.MaxTokens > 393216 {
		payload.MaxTokens = 393216
	}
	if payload.Temperature < 0 {
		payload.Temperature = 0
	}
	if payload.Temperature > 2 {
		payload.Temperature = 2
	}
	var item model.IntegrationAIModel
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if payload.IsDefault {
			if err := tx.Model(&model.IntegrationAIModel{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
				return err
			}
		}
		if payload.ID > 0 {
			if err := tx.First(&item, payload.ID).Error; err != nil {
				return err
			}
			item.Name, item.Provider, item.BaseURL, item.Model = payload.Name, payload.Provider, payload.BaseURL, payload.Model
			item.SystemPrompt, item.Temperature, item.MaxTokens = payload.SystemPrompt, payload.Temperature, payload.MaxTokens
			item.TimeoutSeconds, item.IsDefault, item.Status, item.Description = payload.TimeoutSeconds, payload.IsDefault, payload.Status, payload.Description
			if strings.TrimSpace(payload.APIKey) != "" {
				item.APIKey = strings.TrimSpace(payload.APIKey)
			}
			return tx.Save(&item).Error
		}
		item = model.IntegrationAIModel{Name: payload.Name, Provider: payload.Provider, BaseURL: payload.BaseURL, APIKey: strings.TrimSpace(payload.APIKey), Model: payload.Model,
			SystemPrompt: payload.SystemPrompt, Temperature: payload.Temperature, MaxTokens: payload.MaxTokens, TimeoutSeconds: payload.TimeoutSeconds,
			IsDefault: payload.IsDefault, Status: payload.Status, Description: payload.Description}
		return tx.Create(&item).Error
	})
	if err != nil {
		return nil, err
	}
	return aiModelView(item), nil
}

func (s *Service) DeleteIntegrationAIModel(id uint) error {
	if id == 0 {
		return errors.New("model ID is required")
	}
	var count int64
	if err := s.db.Model(&model.IntegrationAIConversation{}).Where("model_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("model is used by conversations and cannot be deleted directly; disable it first")
	}
	return s.db.Delete(&model.IntegrationAIModel{}, id).Error
}

func (s *Service) TestIntegrationAIModel(payload IntegrationAIModelPayload) (map[string]any, error) {
	var item model.IntegrationAIModel
	if payload.ID > 0 {
		if err := s.db.First(&item, payload.ID).Error; err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(payload.BaseURL) != "" {
		item.BaseURL = strings.TrimRight(strings.TrimSpace(payload.BaseURL), "/")
	}
	if strings.TrimSpace(payload.Model) != "" {
		item.Model = strings.TrimSpace(payload.Model)
	}
	if strings.TrimSpace(payload.APIKey) != "" {
		item.APIKey = strings.TrimSpace(payload.APIKey)
	}
	if payload.TimeoutSeconds > 0 {
		item.TimeoutSeconds = payload.TimeoutSeconds
	}
	if payload.MaxTokens > 0 {
		item.MaxTokens = payload.MaxTokens
	} else if item.MaxTokens <= 0 {
		item.MaxTokens = 2048
	}
	if payload.Temperature >= 0 && payload.Temperature <= 2 {
		item.Temperature = payload.Temperature
	}
	started := time.Now()
	response, err := s.callOpenAICompatible(item, []map[string]any{{"role": "user", "content": "Reply only with OK"}}, nil)
	if err != nil {
		return nil, err
	}
	return map[string]any{"success": true, "latencyMs": time.Since(started).Milliseconds(), "response": response.Content}, nil
}

func (s *Service) ListIntegrationAIConversations(userID uint, keyword string) ([]map[string]any, error) {
	query := s.db.Model(&model.IntegrationAIConversation{}).Where("user_id = ?", userID)
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		query = query.Where("title LIKE ?", "%"+keyword+"%")
	}
	var list []model.IntegrationAIConversation
	if err := query.Order("pinned DESC, COALESCE(last_message_at, created_at) DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	modelNames := map[uint]string{}
	var models []model.IntegrationAIModel
	_ = s.db.Find(&models).Error
	for _, item := range models {
		modelNames[item.ID] = item.Name
	}
	result := make([]map[string]any, 0, len(list))
	for _, item := range list {
		result = append(result, map[string]any{
			"id": item.ID, "title": item.Title, "modelId": item.ModelID, "modelName": modelNames[item.ModelID], "username": item.Username,
			"status": item.Status, "pinned": item.Pinned, "messageCount": item.MessageCount, "lastMessageAt": item.LastMessageAt, "createTime": item.CreatedAt, "updateTime": item.UpdatedAt})
	}
	return result, nil
}

func (s *Service) SaveIntegrationAIConversation(userID uint, username string, payload IntegrationAIConversationPayload) (*model.IntegrationAIConversation, error) {
	title := strings.TrimSpace(payload.Title)
	if title == "" {
		title = "새 대화"
	}
	if payload.ModelID == 0 {
		payload.ModelID = s.defaultAIModelID()
	}
	if payload.ID > 0 {
		var item model.IntegrationAIConversation
		if err := s.db.Where("id = ? AND user_id = ?", payload.ID, userID).First(&item).Error; err != nil {
			return nil, err
		}
		item.Title, item.ModelID, item.Pinned = title, payload.ModelID, payload.Pinned
		if err := s.db.Save(&item).Error; err != nil {
			return nil, err
		}
		return &item, nil
	}
	item := model.IntegrationAIConversation{UserID: userID, Username: username, ModelID: payload.ModelID, Title: title, Status: 1, Pinned: payload.Pinned}
	if err := s.db.Create(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) defaultAIModelID() uint {
	var item model.IntegrationAIModel
	if s.db.Where("status = ?", 1).Order("is_default DESC, id ASC").First(&item).Error == nil {
		return item.ID
	}
	return 0
}

func (s *Service) GetIntegrationAIConversation(userID, id uint) (map[string]any, error) {
	var conversation model.IntegrationAIConversation
	if err := s.db.Where("id = ? AND user_id = ?", id, userID).First(&conversation).Error; err != nil {
		return nil, err
	}
	var messages []model.IntegrationAIMessage
	if err := s.db.Where("conversation_id = ?", id).Order("id ASC").Find(&messages).Error; err != nil {
		return nil, err
	}
	var actions []model.IntegrationAIToolAction
	_ = s.db.Where("conversation_id = ?", id).Order("id ASC").Find(&actions).Error
	for index := range messages {
		messages[index].Content = sanitizeAIMessageContent(messages[index].Content)
	}
	return map[string]any{"conversation": conversation, "messages": messages, "actions": actions}, nil
}

func (s *Service) DeleteIntegrationAIConversation(userID, id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var item model.IntegrationAIConversation
		if err := tx.Where("id = ? AND user_id = ?", id, userID).First(&item).Error; err != nil {
			return err
		}
		if err := tx.Where("conversation_id = ?", id).Delete(&model.IntegrationAIToolAction{}).Error; err != nil {
			return err
		}
		if err := tx.Where("conversation_id = ?", id).Delete(&model.IntegrationAIMessage{}).Error; err != nil {
			return err
		}
		return tx.Delete(&item).Error
	})
}

func (s *Service) SendIntegrationAIChat(userID uint, username string, payload IntegrationAIChatPayload) (map[string]any, error) {
	content := strings.TrimSpace(payload.Content)
	if content == "" {
		return nil, errors.New("conversation content is required")
	}
	var conversation model.IntegrationAIConversation
	if payload.ConversationID == 0 {
		item, err := s.SaveIntegrationAIConversation(userID, username, IntegrationAIConversationPayload{ModelID: payload.ModelID, Title: truncateRunes(content, 40)})
		if err != nil {
			return nil, err
		}
		conversation = *item
	} else if err := s.db.Where("id = ? AND user_id = ?", payload.ConversationID, userID).First(&conversation).Error; err != nil {
		return nil, err
	}
	if payload.ModelID > 0 && payload.ModelID != conversation.ModelID {
		conversation.ModelID = payload.ModelID
		_ = s.db.Model(&conversation).Update("model_id", payload.ModelID).Error
	}
	var aiModel model.IntegrationAIModel
	if err := s.db.Where("id = ? AND status = ?", conversation.ModelID, 1).First(&aiModel).Error; err != nil {
		return nil, errors.New("select an enabled AI model")
	}
	userMessage := model.IntegrationAIMessage{ConversationID: conversation.ID, Role: "user", Content: content, Status: "completed"}
	if err := s.db.Create(&userMessage).Error; err != nil {
		return nil, err
	}
	var history []model.IntegrationAIMessage
	if err := s.db.Where("conversation_id = ? AND role IN ?", conversation.ID, []string{"user", "assistant"}).Order("id DESC").Limit(30).Find(&history).Error; err != nil {
		return nil, err
	}
	sort.Slice(history, func(i, j int) bool { return history[i].ID < history[j].ID })
	messages := []map[string]any{{"role": "system", "content": integrationAISystemPrompt(aiModel.SystemPrompt)}}
	for _, item := range history {
		messages = append(messages, map[string]any{"role": item.Role, "content": sanitizeAIMessageContent(item.Content)})
	}
	tools, configs, err := s.openAIToolDefinitions()
	if err != nil {
		return nil, err
	}
	response, err := s.callOpenAICompatible(aiModel, messages, tools)
	if err != nil {
		return nil, err
	}
	if len(response.ToolCalls) == 0 && hasUnsupportedAIToolProtocol(response.Content) {
		repairMessages := append([]map[string]any{}, messages...)
		repairMessages = append(repairMessages,
			map[string]any{"role": "assistant", "content": response.Content},
			map[string]any{"role": "user", "content": "Internal tool-call markers and XML/DSML must not be exposed. Use only provided native tools; when no tool is available, answer concisely in Korean."},
		)
		response, err = s.callOpenAICompatible(aiModel, repairMessages, tools)
		if err != nil {
			return nil, err
		}
	}
	actions := make([]model.IntegrationAIToolAction, 0)
	finOpsToolUsed := false
	finOpsInstructionAdded := false
	knowledgeBaseToolUsed := false
	knowledgeBaseInstructionAdded := false
	for round := 0; round < 3 && len(response.ToolCalls) > 0; round++ {
		assistantCall := map[string]any{"role": "assistant", "content": response.Content, "tool_calls": response.RawToolCalls}
		messages = append(messages, assistantCall)
		for _, call := range response.ToolCalls {
			config, exists := configs[call.Name]
			if !exists || !config.Enabled {
				continue
			}
			var args map[string]any
			if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
				args = map[string]any{}
			}
			var toolResult any
			if config.RequireConfirmation {
				rawArgs, _ := json.Marshal(args)
				action := model.IntegrationAIToolAction{ConversationID: conversation.ID, UserID: userID, ToolKey: call.Name, ArgumentsJSON: string(rawArgs), Status: "pending"}
				if err := s.db.Create(&action).Error; err != nil {
					return nil, err
				}
				actions = append(actions, action)
				toolResult = map[string]any{"status": "pending_confirmation", "actionId": action.ID, "message": "사용자 확인 후 실행할 수 있습니다."}
			} else {
				toolResult, err = s.executeIntegrationAITool(call.Name, args)
				if err != nil {
					toolResult = map[string]any{"error": err.Error()}
				}
			}
			if call.Name == "finops_cost_analysis" {
				finOpsToolUsed = true
			}
			if call.Name == "knowledge_base_search" {
				knowledgeBaseToolUsed = true
			}
			rawResult, _ := json.Marshal(toolResult)
			messages = append(messages, map[string]any{"role": "tool", "tool_call_id": call.ID, "content": string(rawResult)})
		}
		if finOpsToolUsed && !finOpsInstructionAdded {
			messages = append(messages, map[string]any{"role": "system", "content": finOpsChatResponseInstruction})
			finOpsInstructionAdded = true
		}
		if knowledgeBaseToolUsed && !knowledgeBaseInstructionAdded {
			messages = append(messages, map[string]any{"role": "system", "content": knowledgeBaseChatResponseInstruction})
			knowledgeBaseInstructionAdded = true
		}
		// A knowledge-base lookup already returns the relevant local excerpt.  Do
		// not let a tool-capable model repeatedly issue the same search instead of
		// composing an answer from that excerpt.
		nextTools := tools
		if knowledgeBaseToolUsed {
			nextTools = nil
		}
		response, err = s.callOpenAICompatible(aiModel, messages, nextTools)
		if err != nil {
			return nil, err
		}
	}
	if len(response.ToolCalls) > 0 && strings.TrimSpace(response.Content) == "" {
		response.Content = "Tool 호출이 안전 제한에 도달해 실행을 중단했습니다. Query 범위를 줄여 다시 시도하십시오."
	}
	if strings.TrimSpace(response.Content) == "" {
		response.Content = "작업을 생성했습니다. 아래에서 확인한 뒤 실행하십시오."
	}
	if hasUnsupportedAIToolProtocol(response.Content) {
		response.Content = "Model이 지원되지 않는 내부 Tool 호출 형식을 반환하여 아무 작업도 실행하지 않았습니다. 질문을 다시 작성하거나 Native Tool Calling을 지원하는 Model로 전환하십시오."
	}
	assistantMessage := model.IntegrationAIMessage{ConversationID: conversation.ID, Role: "assistant", Content: response.Content, Status: "completed"}
	if err := s.db.Create(&assistantMessage).Error; err != nil {
		return nil, err
	}
	for i := range actions {
		actions[i].MessageID = assistantMessage.ID
		_ = s.db.Model(&actions[i]).Update("message_id", assistantMessage.ID).Error
	}
	now := time.Now()
	updates := map[string]any{"message_count": gorm.Expr("message_count + ?", 2), "last_message_at": &now}
	if conversation.Title == "새 대화" {
		updates["title"] = truncateRunes(content, 40)
	}
	_ = s.db.Model(&conversation).Updates(updates).Error
	return map[string]any{"conversationId": conversation.ID, "message": assistantMessage, "actions": actions}, nil
}

func truncateRunes(value string, limit int) string {
	chars := []rune(strings.TrimSpace(value))
	if len(chars) <= limit {
		return string(chars)
	}
	return string(chars[:limit]) + "..."
}

func integrationAISystemPrompt(custom string) string {
	base := "당신은 Ops Admin Platform의 DevOps/SRE Assistant입니다. 답변은 한국어로 작성하고 결론, Evidence, 권고 작업 순서로 설명하십시오. 표준 Markdown을 사용하되 내부 XML, DSML, tool_calls, invoke 또는 기타 Tool Protocol을 노출하지 마십시오. Production 변경은 Risk를 명시하고 실제로 실행하지 않은 작업을 실행했다고 주장하지 마십시오. Cloud 비용 Question에는 Local Billing Data만 사용하고 Cloud Provider API를 호출하거나 Billing Data를 실시간 Cloud 상태로 표현하지 마십시오."
	base += "\n\nMonitoring Skill 규칙: PromQL 요청은 Expression을 먼저 생성하고 필요하면 prometheus_query로 검증합니다. Alert 조회는 monitor_alert_event_query, Datasource 조회는 monitor_datasource_query, Host Issue는 host_health_diagnose, 종합 장애는 ops_troubleshooting, Dashboard Issue는 monitor_dashboard_analyze를 우선 사용합니다. 분석은 Tool Evidence를 근거로 하고 확인된 사실과 추정을 구분합니다. Alert 생성은 monitor_alert_rule_draft만 사용하며 사용자 확인 후 비활성 Draft로 저장합니다."
	if strings.TrimSpace(custom) != "" {
		return base + "\n\n추가 지침:\n" + strings.TrimSpace(custom)
	}
	return base
}

func hasUnsupportedAIToolProtocol(content string) bool {
	value := strings.ToLower(content)
	compact := strings.NewReplacer(" ", "", "\n", "", "\t", "").Replace(value)
	return strings.Contains(value, "dsml") || strings.Contains(compact, "<tool_calls>") || strings.Contains(compact, "<invokename=")
}

func sanitizeAIMessageContent(content string) string {
	if !hasUnsupportedAIToolProtocol(content) {
		return content
	}
	return "이 History Message에는 Model이 지원하지 않는 내부 Tool 호출 형식이 포함되어 있어 아무 작업도 실행하지 않았습니다. Query를 다시 실행하십시오."
}

const finOpsChatResponseInstruction = "Cloud Cost Tool이 결과를 반환했습니다. 8줄 이내의 간결한 한국어 Markdown으로 답하십시오. 일반 비용 Question은 Billing Month와 Account, 총 비용, Top 3 Product, Region 요약, 최대 3개 확인 항목을 포함합니다. 실시간 Monitoring Data가 없으면 유휴 상태를 단정하지 말고 검증 필요성을 명시하십시오. Instance 수 또는 Instance별 비용 Question에는 resourceBreakdown을 우선 사용하고 최대 5개 Instance 이름/ID와 비용을 보여주십시오. Data Source가 Local Billing임을 마지막에 명시하십시오."

const knowledgeBaseChatResponseInstruction = "Knowledge Base Search Tool이 Local Markdown Fragment를 반환했습니다. 문서 목차를 나열하지 말고 User Question에 맞게 재구성하십시오. 1~2문장 결론 뒤에 Platform Position, 즉시 사용 가능한 Capability, 권장 사용 경로 순서로 설명하고 구현 완료와 제안을 명확히 구분하십시오. Source 또는 Original Text를 요청한 경우에만 문서 이름이나 Quote를 표시하십시오."
