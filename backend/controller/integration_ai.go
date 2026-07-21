package controller

import (
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"ops-admin/backend/httpx"
	"ops-admin/backend/service"

	"github.com/gin-gonic/gin"
)

func integrationAIUser(c *gin.Context) (uint, string) {
	userID, _ := c.Get("userID")
	username, _ := c.Get("username")
	id, _ := userID.(uint)
	name, _ := username.(string)
	return id, name
}

func (ctl *Controller) GetIntegrationAIModelList(c *gin.Context) {
	data, err := ctl.service.ListIntegrationAIModels()
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) SaveIntegrationAIModel(c *gin.Context) {
	var payload service.IntegrationAIModelPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "无效的模型配置")
		return
	}
	data, err := ctl.service.SaveIntegrationAIModel(payload)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) DeleteIntegrationAIModel(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "无效的模型 ID")
		return
	}
	if err := ctl.service.DeleteIntegrationAIModel(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) TestIntegrationAIModel(c *gin.Context) {
	var payload service.IntegrationAIModelPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "无效的模型配置")
		return
	}
	data, err := ctl.service.TestIntegrationAIModel(payload)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetIntegrationAIConversationList(c *gin.Context) {
	userID, _ := integrationAIUser(c)
	data, err := ctl.service.ListIntegrationAIConversations(userID, c.Query("keyword"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetIntegrationAIConversationDetail(c *gin.Context) {
	userID, _ := integrationAIUser(c)
	id, _ := strconv.ParseUint(c.Query("id"), 10, 64)
	data, err := ctl.service.GetIntegrationAIConversation(userID, uint(id))
	if err != nil {
		httpx.Failed(c, 404, "会话不存在")
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) SaveIntegrationAIConversation(c *gin.Context) {
	userID, username := integrationAIUser(c)
	var payload service.IntegrationAIConversationPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "无效的会话参数")
		return
	}
	data, err := ctl.service.SaveIntegrationAIConversation(userID, username, payload)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) DeleteIntegrationAIConversation(c *gin.Context) {
	userID, _ := integrationAIUser(c)
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "无效的会话 ID")
		return
	}
	if err := ctl.service.DeleteIntegrationAIConversation(userID, payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) SendIntegrationAIChat(c *gin.Context) {
	userID, username := integrationAIUser(c)
	var payload service.IntegrationAIChatPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "无效的对话内容")
		return
	}
	data, err := ctl.service.SendIntegrationAIChat(userID, username, payload)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetIntegrationAIKnowledgeDocumentList(c *gin.Context) {
	data, err := ctl.service.ListIntegrationAIKnowledgeDocuments(c.Query("keyword"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) SaveIntegrationAIKnowledgeDocument(c *gin.Context) {
	var payload service.IntegrationAIKnowledgeDocumentPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "无效的知识库文档参数")
		return
	}
	data, err := ctl.service.SaveIntegrationAIKnowledgeDocument(payload)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) UploadIntegrationAIKnowledgeDocument(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		httpx.Failed(c, 400, "请选择 Markdown 文件")
		return
	}
	if !strings.EqualFold(filepath.Ext(file.Filename), ".md") {
		httpx.Failed(c, 400, "仅支持上传 .md 文件")
		return
	}
	source, err := file.Open()
	if err != nil {
		httpx.Failed(c, 400, "无法读取上传文件")
		return
	}
	defer source.Close()
	content, err := io.ReadAll(io.LimitReader(source, 2*1024*1024+1))
	if err != nil {
		httpx.Failed(c, 400, "读取 Markdown 文件失败")
		return
	}
	if len(content) > 2*1024*1024 {
		httpx.Failed(c, 400, "Markdown 文件不能超过 2MB")
		return
	}
	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		name = strings.TrimSuffix(file.Filename, filepath.Ext(file.Filename))
	}
	data, err := ctl.service.SaveIntegrationAIKnowledgeDocument(service.IntegrationAIKnowledgeDocumentPayload{
		Name: name, FileName: file.Filename, SourceType: "upload", Content: strings.TrimPrefix(string(content), "\ufeff"), Status: 1,
	})
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) DeleteIntegrationAIKnowledgeDocument(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Query("id"), 10, 64)
	if id == 0 {
		httpx.Failed(c, 400, "知识库文档 ID 不能为空")
		return
	}
	if err := ctl.service.DeleteIntegrationAIKnowledgeDocument(uint(id)); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) GetIntegrationAIToolList(c *gin.Context) {
	data, err := ctl.service.ListIntegrationAITools()
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) UpdateIntegrationAITool(c *gin.Context) {
	var payload service.IntegrationAIToolUpdatePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "无效的工具配置")
		return
	}
	if err := ctl.service.UpdateIntegrationAITool(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) ExecuteIntegrationAITool(c *gin.Context) {
	var payload service.IntegrationAIToolExecutePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "无效的工具参数")
		return
	}
	data, err := ctl.service.ExecuteIntegrationAITool(payload)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) ConfirmIntegrationAIAction(c *gin.Context) {
	userID, username := integrationAIUser(c)
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "无效的操作 ID")
		return
	}
	data, err := ctl.service.ConfirmIntegrationAIToolAction(userID, username, payload.ID)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) RejectIntegrationAIAction(c *gin.Context) {
	userID, _ := integrationAIUser(c)
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "无效的操作 ID")
		return
	}
	if err := ctl.service.RejectIntegrationAIToolAction(userID, payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}
