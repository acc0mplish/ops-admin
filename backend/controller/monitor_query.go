package controller

import (
	"strconv"
	"time"

	"ops-admin/backend/httpx"
	"ops-admin/backend/service"

	"github.com/gin-gonic/gin"
)

func (ctl *Controller) QueryMonitorPrometheus(c *gin.Context) {
	var payload struct {
		DatasourceID uint   `json:"datasourceId"`
		Query        string `json:"query"`
		Time         int64  `json:"time"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid query payload")
		return
	}
	ts := time.Now()
	if payload.Time > 0 {
		ts = time.Unix(payload.Time, 0)
	}
	data, err := ctl.service.PrometheusInstantQuery(payload.DatasourceID, payload.Query, ts)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) QueryMonitorPrometheusRange(c *gin.Context) {
	var payload service.MonitorRangeQueryPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid range query payload")
		return
	}
	data, err := ctl.service.MonitorRangeQuery(payload)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) QueryMonitorLogs(c *gin.Context) {
	var payload service.MonitorLogQueryPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid log query payload")
		return
	}
	data, err := ctl.service.QueryMonitorLogs(payload)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetMonitorJaegerServices(c *gin.Context) {
	data, err := ctl.service.ListMonitorJaegerServices(uint(mustAtoi(c.Query("datasourceId"))))
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetMonitorJaegerOperations(c *gin.Context) {
	data, err := ctl.service.ListMonitorJaegerOperations(uint(mustAtoi(c.Query("datasourceId"))), c.Query("service"))
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) QueryMonitorTraces(c *gin.Context) {
	var payload service.MonitorTraceQueryPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid trace query payload")
		return
	}
	data, err := ctl.service.QueryMonitorTraces(payload)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetMonitorTrace(c *gin.Context) {
	data, err := ctl.service.GetMonitorTrace(uint(mustAtoi(c.Query("datasourceId"))), c.Query("traceId"))
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetMonitorElasticsearchIndices(c *gin.Context) {
	data, err := ctl.service.ListMonitorElasticsearchIndices(uint(mustAtoi(c.Query("datasourceId"))))
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetMonitorVictoriaLogsStreams(c *gin.Context) {
	data, err := ctl.service.ListMonitorVictoriaLogsStreams(
		uint(mustAtoi(c.Query("datasourceId"))), c.Query("field"), c.Query("query"),
		monitorQueryInt64(c.Query("startAt")), monitorQueryInt64(c.Query("endAt")), mustAtoi(c.Query("limit")),
	)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetMonitorLogFields(c *gin.Context) {
	data, err := ctl.service.ListMonitorLogFields(
		uint(mustAtoi(c.Query("datasourceId"))), c.Query("index"), c.Query("query"),
		monitorQueryInt64(c.Query("startAt")), monitorQueryInt64(c.Query("endAt")),
	)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetMonitorLogFieldValues(c *gin.Context) {
	data, err := ctl.service.ListMonitorLogFieldValues(
		uint(mustAtoi(c.Query("datasourceId"))), c.Query("index"), c.Query("field"), c.Query("query"),
		monitorQueryInt64(c.Query("startAt")), monitorQueryInt64(c.Query("endAt")), mustAtoi(c.Query("limit")),
	)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func monitorQueryInt64(value string) int64 {
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed
}

func (ctl *Controller) GetMonitorLogShortcuts(c *gin.Context) {
	data, err := ctl.service.ListMonitorLogShortcutsByType(c.GetString("username"), c.Query("datasourceType"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) SaveMonitorLogShortcut(c *gin.Context) {
	var payload service.MonitorLogShortcutPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid log shortcut payload")
		return
	}
	if err := ctl.service.SaveMonitorLogShortcutByType(c.GetString("username"), payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) DeleteMonitorLogShortcut(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid log shortcut payload")
		return
	}
	if err := ctl.service.DeleteMonitorLogShortcut(c.GetString("username"), payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) GetMonitorQueryHistoryList(c *gin.Context) {
	pageNum, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	data, err := ctl.service.ListMonitorQueryHistories(pageNum, pageSize, c.Query("keyword"), c.Query("status"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}
