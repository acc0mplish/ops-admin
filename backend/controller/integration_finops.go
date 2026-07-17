package controller

import (
	"strconv"
	"strings"
	"time"

	"ops-admin/backend/httpx"
	"ops-admin/backend/service"

	"github.com/gin-gonic/gin"
)

func finOpsID(c *gin.Context) uint {
	value := c.Query("id")
	if value == "" {
		value = c.Query("accountId")
	}
	id, _ := strconv.ParseUint(value, 10, 64)
	return uint(id)
}

func finOpsDateRange(c *gin.Context) (time.Time, time.Time, error) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 1, 0)
	var err error
	if value := strings.TrimSpace(c.Query("month")); value != "" {
		start, err = time.ParseInLocation("2006-01", value, time.Local)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		return start, start.AddDate(0, 1, 0), nil
	}
	if value := c.Query("start"); value != "" {
		start, err = time.ParseInLocation("2006-01-02", value, time.Local)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	if value := c.Query("end"); value != "" {
		end, err = time.ParseInLocation("2006-01-02", value, time.Local)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		end = end.AddDate(0, 0, 1)
	}
	return start, end, nil
}

func (ctl *Controller) GetFinOpsAccountList(c *gin.Context) {
	data, err := ctl.service.ListFinOpsAccounts(c.Query("keyword"), c.Query("provider"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) SaveFinOpsAccount(c *gin.Context) {
	var payload service.FinOpsAccountPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "无效的云账号配置")
		return
	}
	data, err := ctl.service.SaveFinOpsAccount(payload)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) DeleteFinOpsAccount(c *gin.Context) {
	if err := ctl.service.DeleteFinOpsAccount(finOpsID(c)); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) TestFinOpsAccount(c *gin.Context) {
	var payload service.FinOpsAccountPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "无效的云账号配置")
		return
	}
	data, err := ctl.service.TestFinOpsAccount(payload)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetFinOpsDashboard(c *gin.Context) {
	start, end, err := finOpsDateRange(c)
	if err != nil {
		httpx.Failed(c, 400, "日期范围格式无效")
		return
	}
	data, err := ctl.service.FinOpsDashboard(start, end)
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetFinOpsBreakdown(c *gin.Context) {
	start, end, err := finOpsDateRange(c)
	if err != nil {
		httpx.Failed(c, 400, "日期范围格式无效")
		return
	}
	accountID, _ := strconv.ParseUint(c.Query("accountId"), 10, 64)
	data, err := ctl.service.FinOpsBreakdown(start, end, c.DefaultQuery("dimension", "provider"), uint(accountID))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetFinOpsResources(c *gin.Context) {
	start, end, err := finOpsDateRange(c)
	if err != nil {
		httpx.Failed(c, 400, "日期范围格式无效")
		return
	}
	accountID, _ := strconv.ParseUint(c.Query("accountId"), 10, 64)
	data, err := ctl.service.FinOpsResources(start, end, uint(accountID))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetFinOpsRecommendationList(c *gin.Context) {
	data, err := ctl.service.ListFinOpsRecommendations(c.Query("status"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GenerateFinOpsRecommendations(c *gin.Context) {
	count, err := ctl.service.GenerateFinOpsRecommendations()
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, gin.H{"count": count})
}

func (ctl *Controller) UpdateFinOpsRecommendation(c *gin.Context) {
	var payload struct {
		ID     uint   `json:"id"`
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "无效的建议状态")
		return
	}
	if err := ctl.service.UpdateFinOpsRecommendation(payload.ID, payload.Status); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) GetFinOpsSyncLogs(c *gin.Context) {
	data, err := ctl.service.ListFinOpsSyncLogs(finOpsID(c))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) TriggerFinOpsSync(c *gin.Context) {
	var payload struct {
		AccountID uint `json:"accountId"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "请选择云账号")
		return
	}
	data, err := ctl.service.SyncFinOpsAccount(payload.AccountID, "manual")
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) ImportFinOpsCosts(c *gin.Context) {
	var payload service.FinOpsCostImportPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "账单 JSON 格式无效")
		return
	}
	data, err := ctl.service.ImportFinOpsCosts(payload)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}
