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
		httpx.Failed(c, 400, "invalid cloud account configuration")
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
		httpx.Failed(c, 400, "invalid cloud account configuration")
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
		httpx.Failed(c, 400, "invalid date range format")
		return
	}
	accountID, _ := strconv.ParseUint(c.Query("account_id"), 10, 64)
	data, err := ctl.service.FinOpsDashboard(start, end, uint(accountID))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetFinOpsBreakdown(c *gin.Context) {
	month := strings.TrimSpace(c.Query("month"))
	if month == "" {
		httpx.Failed(c, 400, "month is required and must use YYYY-MM format")
		return
	}
	start, err := time.ParseInLocation("2006-01", month, time.Local)
	if err != nil {
		httpx.Failed(c, 400, "invalid month format; expected YYYY-MM")
		return
	}
	end := start.AddDate(0, 1, 0)
	accountID, _ := strconv.ParseUint(c.Query("account_id"), 10, 64)
	data, err := ctl.service.FinOpsBreakdown(start, end, c.DefaultQuery("dimension", "provider"), uint(accountID))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetFinOpsLatestBreakdownMonth(c *gin.Context) {
	accountID, _ := strconv.ParseUint(c.Query("account_id"), 10, 64)
	month, err := ctl.service.LatestFinOpsBreakdownMonth(uint(accountID))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, gin.H{"month": month})
}

func (ctl *Controller) GetFinOpsResources(c *gin.Context) {
	accountText := c.Query("account_id")
	if accountText == "" {
		accountText = c.Query("accountId")
	}
	accountID, _ := strconv.ParseUint(accountText, 10, 64)
	if accountID == 0 || strings.TrimSpace(c.Query("start")) == "" || strings.TrimSpace(c.Query("end")) == "" {
		httpx.Failed(c, 400, "select a cloud account, start date, and end date before requesting resource breakdown")
		return
	}
	start, end, err := finOpsDateRange(c)
	if err != nil {
		httpx.Failed(c, 400, "invalid date range format")
		return
	}
	filters := func(key string) []string {
		values := c.QueryArray(key)
		if len(values) == 0 && c.Query(key) != "" {
			values = strings.Split(c.Query(key), ",")
		}
		result := make([]string, 0, len(values))
		for _, value := range values {
			if value = strings.TrimSpace(value); value != "" {
				result = append(result, value)
			}
		}
		return result
	}
	data, err := ctl.service.FinOpsResources(start, end, uint(accountID), filters("region"), filters("resource_type"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetFinOpsRecommendationList(c *gin.Context) {
	accountID, _ := strconv.ParseUint(c.Query("account_id"), 10, 64)
	data, err := ctl.service.ListFinOpsRecommendations(c.Query("status"), uint(accountID), c.Query("month"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GenerateFinOpsRecommendations(c *gin.Context) {
	var payload service.FinOpsRecommendationGeneratePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid recommendation generation payload")
		return
	}
	count, mode, err := ctl.service.GenerateFinOpsRecommendations(payload.ModelID, payload.Strategy, payload.AccountID, payload.Month)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, gin.H{"count": count, "mode": mode})
}

func (ctl *Controller) UpdateFinOpsRecommendation(c *gin.Context) {
	var payload struct {
		ID     uint   `json:"id"`
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid recommendation status payload")
		return
	}
	if err := ctl.service.UpdateFinOpsRecommendation(payload.ID, payload.Status); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) DeleteFinOpsRecommendation(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Query("id"), 10, 64)
	if err := ctl.service.DeleteFinOpsRecommendation(uint(id)); err != nil {
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
		AccountID  uint   `json:"accountId"`
		StartMonth string `json:"start_month"`
		EndMonth   string `json:"end_month"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "select a cloud account")
		return
	}
	now := time.Now()
	if strings.TrimSpace(payload.StartMonth) == "" {
		payload.StartMonth = now.AddDate(0, -5, 0).Format("2006-01")
	}
	if strings.TrimSpace(payload.EndMonth) == "" {
		payload.EndMonth = now.Format("2006-01")
	}
	data, err := ctl.service.SyncFinOpsAccountMonths(payload.AccountID, "manual", payload.StartMonth, payload.EndMonth)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) ImportFinOpsCosts(c *gin.Context) {
	var payload service.FinOpsCostImportPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid billing JSON payload")
		return
	}
	data, err := ctl.service.ImportFinOpsCosts(payload)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}
