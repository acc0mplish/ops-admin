package controller

import (
	"strconv"
	"strings"

	"ops-admin/backend/httpx"
	"ops-admin/backend/service"

	"github.com/gin-gonic/gin"
)

func (ctl *Controller) GetMonitorAlertTemplateList(c *gin.Context) {
	pageNum, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	data, err := ctl.service.ListMonitorAlertTemplates(pageNum, pageSize, c.Query("keyword"), c.Query("category"), c.Query("datasourceType"), c.Query("source"), uint(mustAtoi(c.Query("groupId"))))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetMonitorAlertTemplateGroups(c *gin.Context) {
	data, err := ctl.service.ListMonitorAlertTemplateGroups()
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}
func (ctl *Controller) SaveMonitorAlertTemplateGroup(c *gin.Context) {
	var payload service.MonitorAlertTemplateGroupPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid template group payload")
		return
	}
	data, err := ctl.service.SaveMonitorAlertTemplateGroup(payload)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}
func (ctl *Controller) DeleteMonitorAlertTemplateGroup(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid delete payload")
		return
	}
	if err := ctl.service.DeleteMonitorAlertTemplateGroup(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) GetMonitorAlertTemplateInfo(c *gin.Context) {
	data, err := ctl.service.GetMonitorAlertTemplate(uint(mustAtoi(c.Query("id"))))
	if err != nil {
		httpx.Failed(c, 404, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) SaveMonitorAlertTemplate(c *gin.Context) {
	var payload service.MonitorAlertTemplatePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid alert template payload")
		return
	}
	data, err := ctl.service.SaveMonitorAlertTemplate(payload)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) DeleteMonitorAlertTemplate(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid delete payload")
		return
	}
	if err := ctl.service.DeleteMonitorAlertTemplate(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) ParsePrometheusAlertTemplates(c *gin.Context) {
	var payload struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "Prometheus rule YAML is required")
		return
	}
	content := strings.TrimSpace(payload.Content)
	if content == "" {
		httpx.Failed(c, 400, "Prometheus rule YAML is required")
		return
	}
	if len([]byte(content)) > 2*1024*1024 {
		httpx.Failed(c, 400, "YAML content must not exceed 2 MB")
		return
	}
	data, err := service.ParsePrometheusAlertTemplates([]byte(content))
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) ImportPrometheusAlertTemplates(c *gin.Context) {
	var payload service.MonitorAlertTemplateImportPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid Prometheus template import payload")
		return
	}
	data, err := ctl.service.ImportPrometheusAlertTemplates(payload)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) ExportPrometheusAlertTemplates(c *gin.Context) {
	var payload service.MonitorAlertTemplateExportPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid alert template export payload")
		return
	}
	content, err := ctl.service.ExportPrometheusAlertTemplates(payload.IDs)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	c.Header("Content-Type", "application/x-yaml; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=ops-admin-alert-templates.yaml")
	c.Data(200, "application/x-yaml; charset=utf-8", content)
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

func (ctl *Controller) PreviewMonitorSilenceRule(c *gin.Context) {
	var payload service.MonitorSilenceRulePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid silence rule preview payload")
		return
	}
	data, err := ctl.service.PreviewMonitorSilenceRule(payload)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
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
