package controller

import (
	"strconv"

	"ops-admin/backend/httpx"
	"ops-admin/backend/service"

	"github.com/gin-gonic/gin"
)

func (ctl *Controller) GetOpsJobList(c *gin.Context) {
	pageNum, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	data, err := ctl.service.ListOpsJobs(pageNum, pageSize, c.Query("keyword"), c.Query("status"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetOpsJobInfo(c *gin.Context) {
	data, err := ctl.service.GetOpsJobForView(uint(mustAtoi(c.Query("id"))))
	if err != nil {
		httpx.Failed(c, 404, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) CreateOpsJob(c *gin.Context) {
	var payload service.OpsJobPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid job payload")
		return
	}
	if err := ctl.service.CreateOpsJob(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) UpdateOpsJob(c *gin.Context) {
	var payload service.OpsJobPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid job payload")
		return
	}
	if err := ctl.service.UpdateOpsJob(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) DeleteOpsJob(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid delete payload")
		return
	}
	if err := ctl.service.DeleteOpsJob(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) UpdateOpsJobStatus(c *gin.Context) {
	var payload service.OpsJobStatusPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid job status payload")
		return
	}
	if err := ctl.service.UpdateOpsJobStatus(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) RunOpsJob(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid run payload")
		return
	}
	if err := ctl.service.RunOpsJob(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) GetOpsJobHistoryList(c *gin.Context) {
	pageNum, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	data, err := ctl.service.ListOpsJobHistories(pageNum, pageSize, c.Query("keyword"), c.Query("status"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetOpsJobHistoryDetail(c *gin.Context) {
	data, err := ctl.service.GetOpsJobHistoryDetail(uint(mustAtoi(c.Query("id"))))
	if err != nil {
		httpx.Failed(c, 404, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) ApproveOpsJobHistory(c *gin.Context) {
	var payload service.OpsJobHistoryApprovalPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid approval payload")
		return
	}
	if err := ctl.service.ApproveOpsJobHistoryStep(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) RejectOpsJobHistory(c *gin.Context) {
	var payload service.OpsJobHistoryApprovalPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid approval payload")
		return
	}
	if err := ctl.service.RejectOpsJobHistoryStep(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) GetOpsJobTemplateList(c *gin.Context) {
	pageNum, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	data, err := ctl.service.ListOpsJobTemplates(pageNum, pageSize, c.Query("keyword"), c.Query("status"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetOpsJobTemplateOptions(c *gin.Context) {
	data, err := ctl.service.ListOpsJobTemplateOptions()
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetOpsJobTemplateInfo(c *gin.Context) {
	data, err := ctl.service.GetOpsJobTemplateForView(uint(mustAtoi(c.Query("id"))))
	if err != nil {
		httpx.Failed(c, 404, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) CreateOpsJobTemplate(c *gin.Context) {
	var payload service.OpsJobTemplatePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid job template payload")
		return
	}
	if err := ctl.service.CreateOpsJobTemplate(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) UpdateOpsJobTemplate(c *gin.Context) {
	var payload service.OpsJobTemplatePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid job template payload")
		return
	}
	if err := ctl.service.UpdateOpsJobTemplate(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) DeleteOpsJobTemplate(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid delete payload")
		return
	}
	if err := ctl.service.DeleteOpsJobTemplate(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) UpdateOpsJobTemplateStatus(c *gin.Context) {
	var payload service.OpsJobStatusPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid job template status payload")
		return
	}
	if err := ctl.service.UpdateOpsJobTemplateStatus(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}
