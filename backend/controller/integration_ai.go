package controller

import (
	"strconv"

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
