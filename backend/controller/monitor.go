package controller

import (
	"strconv"
	"strings"
	"time"

	"ops-admin/backend/httpx"
	"ops-admin/backend/service"

	"github.com/gin-gonic/gin"
)

func (ctl *Controller) GetMonitorOverview(c *gin.Context) {
	startAt, err := parseMonitorOverviewDate(c.Query("startDate"), false)
	if err != nil {
		httpx.Failed(c, 400, "invalid startDate format; expected YYYY-MM-DD")
		return
	}
	endAt, err := parseMonitorOverviewDate(c.Query("endDate"), true)
	if err != nil {
		httpx.Failed(c, 400, "invalid endDate format; expected YYYY-MM-DD")
		return
	}
	if startAt != nil && endAt != nil && !startAt.Before(*endAt) {
		httpx.Failed(c, 400, "start date must be earlier than end date")
		return
	}
	data, err := ctl.service.GetMonitorOverview(startAt, endAt)
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

// GetMonitorCommandCenter returns the compact data set required by the
// monitoring command center. It intentionally aggregates native platform
// assets and monitoring data in one request so the screen has a stable first
// paint and does not fan out into a large number of browser requests.
func (ctl *Controller) GetMonitorCommandCenter(c *gin.Context) {
	data, err := ctl.service.GetMonitorCommandCenter()
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func parseMonitorOverviewDate(value string, endOfDay bool) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return nil, err
	}
	if endOfDay {
		parsed = parsed.AddDate(0, 0, 1)
	}
	return &parsed, nil
}

func (ctl *Controller) GetMonitorDatasourceList(c *gin.Context) {
	pageNum, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	data, err := ctl.service.ListMonitorDatasources(pageNum, pageSize, c.Query("keyword"), c.Query("type"), c.Query("status"), c.Query("env"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetMonitorDatasourceOptions(c *gin.Context) {
	data, err := ctl.service.ListMonitorDatasourceOptions()
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetMonitorDatasourceInfo(c *gin.Context) {
	data, err := ctl.service.GetMonitorDatasource(uint(mustAtoi(c.Query("id"))))
	if err != nil {
		httpx.Failed(c, 404, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) SaveMonitorDatasource(c *gin.Context) {
	var payload service.MonitorDatasourcePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid datasource payload")
		return
	}
	if err := ctl.service.SaveMonitorDatasource(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) DeleteMonitorDatasource(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid delete payload")
		return
	}
	if err := ctl.service.DeleteMonitorDatasource(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) TestMonitorDatasource(c *gin.Context) {
	var payload service.MonitorDatasourcePayload
	_ = c.ShouldBindJSON(&payload)
	if err := ctl.service.TestMonitorDatasource(uint(mustAtoi(c.Query("id"))), payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}
