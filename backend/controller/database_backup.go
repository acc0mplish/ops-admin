package controller

import (
	"net/url"
	"strconv"

	"ops-admin/backend/httpx"
	"ops-admin/backend/service"

	"github.com/gin-gonic/gin"
)

func (ctl *Controller) GetDatabaseBackupPlanList(c *gin.Context) {
	pageNum, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	data, err := ctl.service.ListDatabaseBackupPlans(pageNum, pageSize, c.Query("keyword"), c.Query("status"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) SaveDatabaseBackupPlan(c *gin.Context) {
	var payload service.DatabaseBackupPlanPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid backup plan payload")
		return
	}
	if err := ctl.service.SaveDatabaseBackupPlan(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) DeleteDatabaseBackupPlan(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid delete payload")
		return
	}
	if err := ctl.service.DeleteDatabaseBackupPlan(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) RunDatabaseBackupPlan(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid run payload")
		return
	}
	data, err := ctl.service.RunDatabaseBackup(payload.ID, "manual", c.GetString("username"))
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) RunManualDatabaseBackup(c *gin.Context) {
	var payload service.DatabaseManualBackupPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid backup payload")
		return
	}
	payload.Operator = c.GetString("username")
	data, err := ctl.service.RunManualDatabaseBackup(payload)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetDatabaseBackupRecordList(c *gin.Context) {
	pageNum, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	data, err := ctl.service.ListDatabaseBackupRecords(
		pageNum,
		pageSize,
		uint(mustAtoi(c.Query("databaseId"))),
		c.Query("status"),
		c.Query("triggerType"),
	)
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) DownloadDatabaseBackup(c *gin.Context) {
	data, filename, err := ctl.service.GetDatabaseBackupFile(uint(mustAtoi(c.Query("id"))))
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	c.Header("Content-Disposition", "attachment; filename*=UTF-8''"+url.QueryEscape(filename))
	c.Data(200, "application/sql; charset=utf-8", data)
}
