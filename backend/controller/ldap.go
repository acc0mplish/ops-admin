package controller

import (
	"net/http"

	"ops-admin/backend/httpx"
	"ops-admin/backend/service"

	"github.com/gin-gonic/gin"
)

func (ctl *Controller) GetLDAPConfig(c *gin.Context) {
	data, err := ctl.service.GetLDAPConfig()
	if err != nil {
		httpx.Failed(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) SaveLDAPConfig(c *gin.Context) {
	var payload service.LDAPConfigPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, http.StatusBadRequest, "invalid LDAP config payload")
		return
	}
	data, err := ctl.service.SaveLDAPConfig(payload)
	if err != nil {
		httpx.Failed(c, http.StatusBadRequest, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) TestLDAPConfig(c *gin.Context) {
	var payload service.LDAPConfigPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, http.StatusBadRequest, "invalid LDAP config payload")
		return
	}
	if err := ctl.service.TestLDAPConfig(payload); err != nil {
		httpx.Failed(c, http.StatusBadRequest, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) PreviewLDAPUsers(c *gin.Context) {
	data, err := ctl.service.PreviewLDAPUsers(c.Query("keyword"))
	if err != nil {
		httpx.Failed(c, http.StatusBadRequest, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) SyncLDAPUsers(c *gin.Context) {
	var payload service.LDAPSyncPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, http.StatusBadRequest, "invalid LDAP sync payload")
		return
	}
	data, err := ctl.service.SyncLDAPUsers(payload)
	if err != nil {
		httpx.Failed(c, http.StatusBadRequest, err.Error())
		return
	}
	httpx.Success(c, data)
}
