package controller

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"ops-admin/backend/httpx"
	"ops-admin/backend/service"
)

func (ctl *Controller) GetAssetServiceDiagnosisProcesses(c *gin.Context) {
	var target service.AssetServiceDiagnosisTarget
	if err := c.ShouldBindQuery(&target); err != nil {
		httpx.FailedCode(c, 400, "INVALID_DIAGNOSIS_TARGET", nil)
		return
	}
	data, err := ctl.service.GetAssetServiceDiagnosisProcesses(target)
	if err != nil {
		httpx.FailedError(c, 400, err)
		return
	}
	httpx.Success(c, data)
}
func (ctl *Controller) GetAssetServiceDiagnosisEnvironment(c *gin.Context) {
	var target service.AssetServiceDiagnosisTarget
	if err := c.ShouldBindQuery(&target); err != nil {
		httpx.FailedCode(c, 400, "INVALID_DIAGNOSIS_TARGET", nil)
		return
	}
	data, err := ctl.service.GetAssetServiceDiagnosisEnvironment(target)
	if err != nil {
		httpx.FailedError(c, 400, err)
		return
	}
	httpx.Success(c, data)
}
// RunAssetServiceDiagnosis answers the POST replacement route of the §4.9
// mutating-GET conversion; the target struct carries both json and form tags,
// so only the binding changed and the service logic is untouched.
func (ctl *Controller) RunAssetServiceDiagnosis(c *gin.Context) {
	var target service.AssetServiceDiagnosisTarget
	if err := c.ShouldBindJSON(&target); err != nil {
		httpx.FailedCode(c, 400, "INVALID_DIAGNOSIS_TARGET", nil)
		return
	}
	data, err := ctl.service.RunAssetServiceDiagnosis(target)
	if err != nil {
		httpx.FailedError(c, 400, err)
		return
	}
	httpx.Success(c, data)
}

// AssetServiceDiagnosisRunGone answers the retired GET form of the diagnosis
// run endpoint for one release with 410 Gone plus headers naming the POST
// replacement — external scripts and bookmarks are the remaining audience.
func (ctl *Controller) AssetServiceDiagnosisRunGone(c *gin.Context) {
	c.Header("Allow", http.MethodPost)
	c.Header("Location", "/api/v1/asset/service/diagnosis/run")
	httpx.FailedCode(c, http.StatusGone, "DIAGNOSIS_RUN_MOVED_TO_POST", nil)
}
func (ctl *Controller) DownloadAssetServiceArthas(c *gin.Context) {
	var target service.AssetServiceDiagnosisTarget
	if err := c.ShouldBindJSON(&target); err != nil {
		httpx.FailedCode(c, 400, "INVALID_DIAGNOSIS_TARGET", nil)
		return
	}
	data, err := ctl.service.DownloadAssetServiceArthas(target)
	if err != nil {
		httpx.FailedError(c, 400, err)
		return
	}
	httpx.Success(c, data)
}
func (ctl *Controller) UploadAssetServiceArthas(c *gin.Context) {
	var target service.AssetServiceDiagnosisTarget
	if err := c.ShouldBind(&target); err != nil {
		httpx.FailedCode(c, 400, "INVALID_DIAGNOSIS_TARGET", nil)
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		httpx.FailedCode(c, 400, "ARTHAS_FILE_REQUIRED", nil)
		return
	}
	source, err := file.Open()
	if err != nil {
		httpx.FailedError(c, 400, err)
		return
	}
	defer source.Close()
	content, err := io.ReadAll(source)
	if err != nil {
		httpx.FailedError(c, 400, err)
		return
	}
	data, err := ctl.service.UploadAssetServiceArthas(target, content)
	if err != nil {
		httpx.FailedError(c, 400, err)
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetAssetServiceList(c *gin.Context) {
	data, err := ctl.service.ListAssetServices(mustAtoi(c.DefaultQuery("pageNum", "1")), mustAtoi(c.DefaultQuery("pageSize", "20")), c.Query("keyword"))
	if err != nil {
		httpx.FailedError(c, 500, err)
		return
	}
	httpx.Success(c, data)
}
func (ctl *Controller) GetAssetServiceInfo(c *gin.Context) {
	data, err := ctl.service.GetAssetService(uint(mustAtoi(c.Query("id"))))
	if err != nil {
		httpx.FailedError(c, 404, err)
		return
	}
	httpx.Success(c, data)
}
func (ctl *Controller) SaveAssetService(c *gin.Context) {
	var payload service.AssetServicePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.FailedCode(c, http.StatusBadRequest, "INVALID_ASSET_SERVICE_PAYLOAD", nil)
		return
	}
	if err := ctl.service.SaveAssetService(payload); err != nil {
		httpx.FailedError(c, http.StatusBadRequest, err)
		return
	}
	httpx.Success(c, true)
}
func (ctl *Controller) DeleteAssetService(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.FailedCode(c, http.StatusBadRequest, "INVALID_DELETE_PAYLOAD", nil)
		return
	}
	if err := ctl.service.DeleteAssetService(payload.ID); err != nil {
		httpx.FailedError(c, http.StatusBadRequest, err)
		return
	}
	httpx.Success(c, true)
}
func (ctl *Controller) GetAssetServiceK8sCatalog(c *gin.Context) {
	data, err := ctl.service.GetAssetServiceK8sCatalog(uint(mustAtoi(c.Query("clusterId"))), c.Query("namespace"))
	if err != nil {
		httpx.FailedError(c, http.StatusBadRequest, err)
		return
	}
	httpx.Success(c, data)
}
func (ctl *Controller) GetAssetServiceRuntimeTopology(c *gin.Context) {
	data, err := ctl.service.GetAssetServiceRuntimeTopology(uint(mustAtoi(c.Query("serviceId"))))
	if err != nil {
		httpx.FailedError(c, http.StatusBadRequest, err)
		return
	}
	httpx.Success(c, data)
}
func (ctl *Controller) GetAssetServiceWorkloadRuntime(c *gin.Context) {
	data, err := ctl.service.GetAssetServiceWorkloadRuntime(uint(mustAtoi(c.Query("serviceId"))), c.Query("workloadType"), c.Query("workloadName"))
	if err != nil {
		httpx.FailedError(c, http.StatusBadRequest, err)
		return
	}
	httpx.Success(c, data)
}
func (ctl *Controller) GetAssetServiceWorkloadTopology(c *gin.Context) {
	data, err := ctl.service.GetAssetServiceWorkloadTopology(uint(mustAtoi(c.Query("serviceId"))), c.Query("workloadType"), c.Query("workloadName"))
	if err != nil {
		httpx.FailedError(c, http.StatusBadRequest, err)
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetAssetServiceWorkloadMetrics(c *gin.Context) {
	data, err := ctl.service.GetAssetServiceWorkloadMetrics(uint(mustAtoi(c.Query("serviceId"))), c.Query("workloadType"), c.Query("workloadName"), c.Query("range"))
	if err != nil {
		httpx.FailedError(c, http.StatusBadRequest, err)
		return
	}
	httpx.Success(c, data)
}
func (ctl *Controller) GetAssetServiceWorkloadRolloutHistory(c *gin.Context) {
	data, err := ctl.service.GetAssetServiceWorkloadRolloutHistory(uint(mustAtoi(c.Query("serviceId"))), c.Query("workloadType"), c.Query("workloadName"))
	if err != nil {
		httpx.FailedError(c, http.StatusBadRequest, err)
		return
	}
	httpx.Success(c, data)
}
func (ctl *Controller) RollbackAssetServiceWorkload(c *gin.Context) {
	var payload service.AssetServiceWorkloadRollbackPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.FailedCode(c, http.StatusBadRequest, "INVALID_WORKLOAD_ROLLBACK_PAYLOAD", nil)
		return
	}
	data, err := ctl.service.RollbackAssetServiceWorkload(payload)
	if err != nil {
		httpx.FailedError(c, http.StatusBadRequest, err)
		return
	}
	httpx.Success(c, data)
}
func (ctl *Controller) GetAssetServiceWorkloadLogs(c *gin.Context) {
	data, err := ctl.service.GetAssetServiceWorkloadLogs(uint(mustAtoi(c.Query("serviceId"))), c.Query("workloadType"), c.Query("workloadName"), c.Query("podName"), c.Query("container"), mustAtoi(c.DefaultQuery("tailLines", "200")))
	if err != nil {
		httpx.FailedError(c, http.StatusBadRequest, err)
		return
	}
	httpx.Success(c, data)
}
