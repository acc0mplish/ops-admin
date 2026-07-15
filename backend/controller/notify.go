package controller

import (
	"strconv"

	"ops-admin/backend/httpx"
	"ops-admin/backend/service"

	"github.com/gin-gonic/gin"
)

func (ctl *Controller) GetNotifyTemplateList(c *gin.Context) {
	pageNum, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	data, err := ctl.service.ListNotifyTemplates(pageNum, pageSize, c.Query("keyword"), c.Query("channelType"), c.Query("scope"), c.Query("status"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetNotifyTemplateOptions(c *gin.Context) {
	data, err := ctl.service.ListNotifyTemplateOptions(c.Query("channelType"), c.Query("scope"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetNotifyTemplateInfo(c *gin.Context) {
	data, err := ctl.service.GetNotifyTemplate(uint(mustAtoi(c.Query("id"))))
	if err != nil {
		httpx.Failed(c, 404, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) SaveNotifyTemplate(c *gin.Context) {
	var payload service.NotifyTemplatePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid notification template payload")
		return
	}
	if err := ctl.service.SaveNotifyTemplate(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) DeleteNotifyTemplate(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid delete payload")
		return
	}
	if err := ctl.service.DeleteNotifyTemplate(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) GetNotifyChannelList(c *gin.Context) {
	pageNum, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	data, err := ctl.service.ListNotifyChannels(pageNum, pageSize, c.Query("keyword"), c.Query("channelType"), c.Query("status"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetNotifyChannelOptions(c *gin.Context) {
	data, err := ctl.service.ListNotifyChannelOptions(c.Query("channelType"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetNotifyChannelInfo(c *gin.Context) {
	data, err := ctl.service.GetNotifyChannel(uint(mustAtoi(c.Query("id"))))
	if err != nil {
		httpx.Failed(c, 404, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) SaveNotifyChannel(c *gin.Context) {
	var payload service.NotifyChannelPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid notification channel payload")
		return
	}
	if err := ctl.service.SaveNotifyChannel(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) DeleteNotifyChannel(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid delete payload")
		return
	}
	if err := ctl.service.DeleteNotifyChannel(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) GetNotifyRuleList(c *gin.Context) {
	pageNum, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	data, err := ctl.service.ListNotifyRules(pageNum, pageSize, c.Query("keyword"), c.Query("scope"), c.Query("status"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetNotifyRuleOptions(c *gin.Context) {
	data, err := ctl.service.ListNotifyRuleOptions(c.Query("scope"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetNotifyRuleInfo(c *gin.Context) {
	data, err := ctl.service.GetNotifyRule(uint(mustAtoi(c.Query("id"))))
	if err != nil {
		httpx.Failed(c, 404, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) SaveNotifyRule(c *gin.Context) {
	var payload service.NotifyRulePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid notification rule payload")
		return
	}
	if err := ctl.service.SaveNotifyRule(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) DeleteNotifyRule(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid delete payload")
		return
	}
	if err := ctl.service.DeleteNotifyRule(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) TestNotifyRule(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil || payload.ID == 0 {
		httpx.Failed(c, 400, "invalid notification rule test payload")
		return
	}
	data, err := ctl.service.TestNotifyRule(payload.ID)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetNotifySendLogList(c *gin.Context) {
	pageNum, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	data, err := ctl.service.ListNotifySendLogs(
		pageNum,
		pageSize,
		c.Query("keyword"),
		c.Query("status"),
		c.Query("channelType"),
		c.Query("scope"),
		c.Query("startTime"),
		c.Query("endTime"),
	)
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) RetryNotifySendLog(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil || payload.ID == 0 {
		httpx.Failed(c, 400, "invalid notification retry payload")
		return
	}
	data, err := ctl.service.RetryNotifySendLog(payload.ID)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}
