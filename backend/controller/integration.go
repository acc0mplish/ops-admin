package controller

import (
	"net/http"
	"strconv"

	"ops-admin/backend/httpx"
	"ops-admin/backend/service"

	"github.com/gin-gonic/gin"
)

func (ctl *Controller) GetIntegrationNavigationGroupList(c *gin.Context) {
	data, err := ctl.service.ListIntegrationNavigationGroups(c.Query("keyword"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) SaveIntegrationNavigationGroup(c *gin.Context) {
	var payload service.IntegrationNavigationGroupPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "无效的导航组参数")
		return
	}
	data, err := ctl.service.SaveIntegrationNavigationGroup(payload)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) DeleteIntegrationNavigationGroup(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "无效的删除参数")
		return
	}
	if err := ctl.service.DeleteIntegrationNavigationGroup(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) RegenerateIntegrationPublicToken(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "无效的导航组参数")
		return
	}
	data, err := ctl.service.RegenerateIntegrationPublicToken(payload.ID)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetIntegrationNavigationList(c *gin.Context) {
	groupID, _ := strconv.ParseUint(c.Query("groupId"), 10, 64)
	data, err := ctl.service.ListIntegrationNavigations(uint(groupID), c.Query("keyword"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) SaveIntegrationNavigation(c *gin.Context) {
	var payload service.IntegrationNavigationPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "无效的导航参数")
		return
	}
	data, err := ctl.service.SaveIntegrationNavigation(payload)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) DeleteIntegrationNavigation(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "无效的删除参数")
		return
	}
	if err := ctl.service.DeleteIntegrationNavigation(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) GetPublicIntegrationNavigation(c *gin.Context) {
	data, err := ctl.service.GetPublicIntegrationNavigation(c.Param("token"))
	if err != nil {
		httpx.Failed(c, http.StatusNotFound, "公开导航不存在或已关闭")
		return
	}
	httpx.Success(c, data)
}
