package controller

import (
	"strconv"

	"ops-admin/backend/httpx"
	"ops-admin/backend/service"

	"github.com/gin-gonic/gin"
)

func (ctl *Controller) GetAssetGatewayList(c *gin.Context) {
	pageNum, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	data, err := ctl.service.ListAssetGateways(pageNum, pageSize, c.Query("keyword"), c.Query("status"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetAssetGatewayOptions(c *gin.Context) {
	data, err := ctl.service.ListAssetGatewayOptions()
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetAssetGatewayInfo(c *gin.Context) {
	data, err := ctl.service.GetAssetGateway(uint(mustAtoi(c.Query("id"))))
	if err != nil {
		httpx.Failed(c, 404, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) CreateAssetGateway(c *gin.Context) {
	var payload service.AssetGatewayPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "Invalid gateway payload")
		return
	}
	if err := ctl.service.CreateAssetGateway(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) UpdateAssetGateway(c *gin.Context) {
	var payload service.AssetGatewayPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "Invalid gateway payload")
		return
	}
	if err := ctl.service.UpdateAssetGateway(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) DeleteAssetGateway(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "Invalid delete payload")
		return
	}
	if err := ctl.service.DeleteAssetGateway(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) UpdateAssetGatewayStatus(c *gin.Context) {
	var payload service.AdminStatusPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "Invalid status payload")
		return
	}
	if err := ctl.service.UpdateAssetGatewayStatus(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) TestAssetGatewayConnection(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "Invalid test payload")
		return
	}
	data, err := ctl.service.TestAssetGatewayConnection(payload.ID)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}
