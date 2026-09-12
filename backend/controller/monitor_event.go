package controller

import (
	"strconv"

	"ops-admin/backend/httpx"
	"ops-admin/backend/service"

	"github.com/gin-gonic/gin"
)

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
