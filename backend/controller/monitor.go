package controller

import (
	"strconv"
	"time"

	"ops-admin/backend/httpx"
	"ops-admin/backend/service"

	"github.com/gin-gonic/gin"
)

func (ctl *Controller) GetMonitorOverview(c *gin.Context) {
	data, err := ctl.service.GetMonitorOverview()
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
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

func (ctl *Controller) GetMonitorElasticsearchIndices(c *gin.Context) {
	data, err := ctl.service.ListMonitorElasticsearchIndices(uint(mustAtoi(c.Query("datasourceId"))))
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetMonitorLogShortcuts(c *gin.Context) {
	data, err := ctl.service.ListMonitorLogShortcuts(c.GetString("username"))
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
	if err := ctl.service.SaveMonitorLogShortcut(c.GetString("username"), payload); err != nil {
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

func (ctl *Controller) GetMonitorAlertRuleList(c *gin.Context) {
	pageNum, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	data, err := ctl.service.ListMonitorAlertRules(pageNum, pageSize, c.Query("keyword"), c.Query("status"), c.Query("severity"), c.Query("env"), c.Query("alertType"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetMonitorAlertRuleInfo(c *gin.Context) {
	data, err := ctl.service.GetMonitorAlertRule(uint(mustAtoi(c.Query("id"))))
	if err != nil {
		httpx.Failed(c, 404, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) SaveMonitorAlertRule(c *gin.Context) {
	var payload service.MonitorAlertRulePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid alert rule payload")
		return
	}
	if err := ctl.service.SaveMonitorAlertRule(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) DeleteMonitorAlertRule(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid delete payload")
		return
	}
	if err := ctl.service.DeleteMonitorAlertRule(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) UpdateMonitorAlertRuleStatus(c *gin.Context) {
	var payload struct {
		ID     uint `json:"id"`
		Status int  `json:"status"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid status payload")
		return
	}
	if err := ctl.service.UpdateMonitorAlertRuleStatus(payload.ID, payload.Status); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) BatchUpdateMonitorAlertRules(c *gin.Context) {
	var payload service.MonitorAlertRuleBatchPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid batch alert rule payload")
		return
	}
	if err := ctl.service.BatchUpdateMonitorAlertRules(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) RunMonitorAlertRule(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid run payload")
		return
	}
	if err := ctl.service.RunMonitorAlertRule(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) PreviewMonitorAlertRule(c *gin.Context) {
	var payload service.MonitorAlertRulePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid alert rule preview payload")
		return
	}
	data, err := ctl.service.PreviewMonitorAlertRule(payload)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetMonitorSilenceRuleList(c *gin.Context) {
	pageNum, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	data, err := ctl.service.ListMonitorSilenceRules(pageNum, pageSize, c.Query("keyword"), c.Query("status"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetMonitorSilenceRuleInfo(c *gin.Context) {
	data, err := ctl.service.GetMonitorSilenceRule(uint(mustAtoi(c.Query("id"))))
	if err != nil {
		httpx.Failed(c, 404, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) SaveMonitorSilenceRule(c *gin.Context) {
	var payload service.MonitorSilenceRulePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid silence rule payload")
		return
	}
	if err := ctl.service.SaveMonitorSilenceRule(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) DeleteMonitorSilenceRule(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid delete payload")
		return
	}
	if err := ctl.service.DeleteMonitorSilenceRule(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) BatchUpdateMonitorSilenceRules(c *gin.Context) {
	var payload service.MonitorRuleBatchPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid silence rule batch payload")
		return
	}
	if err := ctl.service.BatchUpdateMonitorSilenceRules(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) GetMonitorAggregationRuleList(c *gin.Context) {
	pageNum, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	data, err := ctl.service.ListMonitorAggregationRules(pageNum, pageSize, c.Query("keyword"), c.Query("status"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetMonitorAggregationRuleInfo(c *gin.Context) {
	data, err := ctl.service.GetMonitorAggregationRule(uint(mustAtoi(c.Query("id"))))
	if err != nil {
		httpx.Failed(c, 404, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) SaveMonitorAggregationRule(c *gin.Context) {
	var payload service.MonitorAggregationRulePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid aggregation rule payload")
		return
	}
	if err := ctl.service.SaveMonitorAggregationRule(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) DeleteMonitorAggregationRule(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid delete payload")
		return
	}
	if err := ctl.service.DeleteMonitorAggregationRule(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) BatchUpdateMonitorAggregationRules(c *gin.Context) {
	var payload service.MonitorRuleBatchPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid aggregation rule batch payload")
		return
	}
	if err := ctl.service.BatchUpdateMonitorAggregationRules(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) GetMonitorAlertEventList(c *gin.Context) {
	pageNum, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	data, err := ctl.service.ListMonitorAlertEvents(pageNum, pageSize, c.Query("keyword"), c.Query("status"), c.Query("severity"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetMonitorAlertEventDetail(c *gin.Context) {
	data, err := ctl.service.GetMonitorAlertEventDetail(uint(mustAtoi(c.Query("id"))))
	if err != nil {
		httpx.Failed(c, 404, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) ClaimMonitorAlertEvent(c *gin.Context) {
	var payload service.MonitorAlertEventActionPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid claim payload")
		return
	}
	if err := ctl.service.ClaimMonitorAlertEvent(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) ResolveMonitorAlertEvent(c *gin.Context) {
	var payload service.MonitorAlertEventActionPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid resolve payload")
		return
	}
	if err := ctl.service.ResolveMonitorAlertEvent(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) BatchUpdateMonitorAlertEvents(c *gin.Context) {
	var payload service.MonitorAlertEventBatchPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid alert event batch payload")
		return
	}
	if err := ctl.service.BatchUpdateMonitorAlertEvents(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) GetMonitorDashboardList(c *gin.Context) {
	pageNum, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	data, err := ctl.service.ListMonitorDashboards(pageNum, pageSize, c.Query("keyword"), c.Query("status"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetMonitorDashboardInfo(c *gin.Context) {
	data, err := ctl.service.GetMonitorDashboard(uint(mustAtoi(c.Query("id"))))
	if err != nil {
		httpx.Failed(c, 404, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) SaveMonitorDashboard(c *gin.Context) {
	var payload service.MonitorDashboardPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid dashboard payload")
		return
	}
	data, err := ctl.service.SaveMonitorDashboard(payload)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) DeleteMonitorDashboard(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid delete payload")
		return
	}
	if err := ctl.service.DeleteMonitorDashboard(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) SaveMonitorDashboardPanel(c *gin.Context) {
	var payload service.MonitorDashboardPanelPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid panel payload")
		return
	}
	if err := ctl.service.SaveMonitorDashboardPanel(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) DeleteMonitorDashboardPanel(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid delete payload")
		return
	}
	if err := ctl.service.DeleteMonitorDashboardPanel(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) QueryMonitorDashboardPanel(c *gin.Context) {
	var payload service.MonitorDashboardPanelQueryPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid query payload")
		return
	}
	data, err := ctl.service.QueryMonitorDashboardPanel(payload)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}
