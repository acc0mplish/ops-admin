package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"ops-admin/backend/httpx"
	"ops-admin/backend/service"
)

func (ctl *Controller) GetAssetServiceList(c *gin.Context) {
	data, err := ctl.service.ListAssetServices(mustAtoi(c.DefaultQuery("pageNum", "1")), mustAtoi(c.DefaultQuery("pageSize", "20")), c.Query("keyword"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}
func (ctl *Controller) GetAssetServiceInfo(c *gin.Context) {
	data, err := ctl.service.GetAssetService(uint(mustAtoi(c.Query("id"))))
	if err != nil {
		httpx.Failed(c, 404, err.Error())
		return
	}
	httpx.Success(c, data)
}
func (ctl *Controller) SaveAssetService(c *gin.Context) {
	var payload service.AssetServicePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, http.StatusBadRequest, "invalid service payload")
		return
	}
	if err := ctl.service.SaveAssetService(payload); err != nil {
		httpx.Failed(c, http.StatusBadRequest, err.Error())
		return
	}
	httpx.Success(c, true)
}
func (ctl *Controller) DeleteAssetService(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, http.StatusBadRequest, "invalid delete payload")
		return
	}
	if err := ctl.service.DeleteAssetService(payload.ID); err != nil {
		httpx.Failed(c, http.StatusBadRequest, err.Error())
		return
	}
	httpx.Success(c, true)
}
func (ctl *Controller) GetAssetServiceK8sCatalog(c *gin.Context) {
	data, err := ctl.service.GetAssetServiceK8sCatalog(uint(mustAtoi(c.Query("clusterId"))), c.Query("namespace"))
	if err != nil {
		httpx.Failed(c, http.StatusBadRequest, err.Error())
		return
	}
	httpx.Success(c, data)
}
func (ctl *Controller) GetAssetServiceRuntimeTopology(c *gin.Context) {
	data, err := ctl.service.GetAssetServiceRuntimeTopology(uint(mustAtoi(c.Query("serviceId"))))
	if err != nil {
		httpx.Failed(c, http.StatusBadRequest, err.Error())
		return
	}
	httpx.Success(c, data)
}
func (ctl *Controller) GetAssetServiceWorkloadRuntime(c *gin.Context) {
	data, err := ctl.service.GetAssetServiceWorkloadRuntime(uint(mustAtoi(c.Query("serviceId"))), c.Query("workloadType"), c.Query("workloadName"))
	if err != nil {
		httpx.Failed(c, http.StatusBadRequest, err.Error())
		return
	}
	httpx.Success(c, data)
}
func (ctl *Controller) GetAssetServiceWorkloadTopology(c *gin.Context) {
	data, err := ctl.service.GetAssetServiceWorkloadTopology(uint(mustAtoi(c.Query("serviceId"))), c.Query("workloadType"), c.Query("workloadName"))
	if err != nil {
		httpx.Failed(c, http.StatusBadRequest, err.Error())
		return
	}
	httpx.Success(c, data)
}
func (ctl *Controller) GetAssetServiceWorkloadRolloutHistory(c *gin.Context) {
	data, err := ctl.service.GetAssetServiceWorkloadRolloutHistory(uint(mustAtoi(c.Query("serviceId"))), c.Query("workloadType"), c.Query("workloadName"))
	if err != nil {
		httpx.Failed(c, http.StatusBadRequest, err.Error())
		return
	}
	httpx.Success(c, data)
}
func (ctl *Controller) RollbackAssetServiceWorkload(c *gin.Context) {
	var payload service.AssetServiceWorkloadRollbackPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, http.StatusBadRequest, "invalid workload rollback payload")
		return
	}
	data, err := ctl.service.RollbackAssetServiceWorkload(payload)
	if err != nil {
		httpx.Failed(c, http.StatusBadRequest, err.Error())
		return
	}
	httpx.Success(c, data)
}
func (ctl *Controller) GetAssetServiceWorkloadLogs(c *gin.Context) {
	data, err := ctl.service.GetAssetServiceWorkloadLogs(uint(mustAtoi(c.Query("serviceId"))), c.Query("workloadType"), c.Query("workloadName"), c.Query("podName"), c.Query("container"), mustAtoi(c.DefaultQuery("tailLines", "200")))
	if err != nil {
		httpx.Failed(c, http.StatusBadRequest, err.Error())
		return
	}
	httpx.Success(c, data)
}
