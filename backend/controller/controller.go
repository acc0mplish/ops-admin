package controller

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"ops-admin/backend/auth"
	"ops-admin/backend/httpx"
	"ops-admin/backend/model"
	"ops-admin/backend/service"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/xuri/excelize/v2"
)

type Controller struct {
	service *service.Service
}

const refreshCookieName = "ops-admin-refresh"

var terminalUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func New(svc *service.Service) *Controller {
	return &Controller{service: svc}
}

func (ctl *Controller) Ping(c *gin.Context) {
	httpx.Success(c, gin.H{"name": "ops-admin", "status": "ok"})
}

func (ctl *Controller) Login(c *gin.Context) {
	var req service.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Failed(c, http.StatusBadRequest, "invalid login payload")
		return
	}
	data, refreshToken, err := ctl.service.Login(req, c.ClientIP(), c.Request.UserAgent(), runtime.GOOS)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	setRefreshCookie(c, refreshToken, int(auth.SessionMaxTTL.Seconds()))
	httpx.Success(c, data)
}

func (ctl *Controller) RefreshToken(c *gin.Context) {
	var payload struct {
		LastActivityAt int64 `json:"lastActivityAt"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil && err != io.EOF {
		httpx.Failed(c, http.StatusBadRequest, "invalid refresh payload")
		return
	}
	refreshToken, err := c.Cookie(refreshCookieName)
	if err != nil {
		clearRefreshCookie(c)
		httpx.Failed(c, http.StatusUnauthorized, "session expired; sign in again")
		return
	}
	reportedAt := time.Time{}
	if payload.LastActivityAt > 0 {
		reportedAt = time.UnixMilli(payload.LastActivityAt)
	}
	data, nextRefreshToken, err := ctl.service.RefreshSession(refreshToken, reportedAt)
	if err != nil {
		clearRefreshCookie(c)
		httpx.Failed(c, http.StatusUnauthorized, "session expired; sign in again")
		return
	}
	maxAge := int(auth.SessionMaxTTL.Seconds())
	if expiresAt, ok := data["sessionExpiresAt"].(int64); ok {
		maxAge = max(0, int(time.Until(time.UnixMilli(expiresAt)).Seconds()))
	}
	setRefreshCookie(c, nextRefreshToken, maxAge)
	httpx.Success(c, data)
}

func (ctl *Controller) Logout(c *gin.Context) {
	if refreshToken, err := c.Cookie(refreshCookieName); err == nil {
		ctl.service.RevokeSession(refreshToken)
	}
	clearRefreshCookie(c)
	httpx.Success(c, gin.H{"loggedOut": true})
}

func setRefreshCookie(c *gin.Context, token string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	secure := c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
	c.SetCookie(refreshCookieName, token, maxAge, "/api/v1/auth", "", secure, true)
}

func clearRefreshCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	secure := c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
	c.SetCookie(refreshCookieName, "", -1, "/api/v1/auth", "", secure, true)
}

func (ctl *Controller) Profile(c *gin.Context) {
	userID := c.GetUint("userID")
	data, err := ctl.service.GetProfile(userID)
	if err != nil {
		httpx.Failed(c, 404, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetSystemConfig(c *gin.Context) {
	data, err := ctl.service.GetSystemConfig()
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) UpdateSystemConfig(c *gin.Context) {
	var payload service.SystemConfigPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid system config payload")
		return
	}
	data, err := ctl.service.UpdateSystemConfig(payload)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) UploadSystemAsset(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		httpx.Failed(c, 400, "select a file to upload")
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg":
	default:
		httpx.Failed(c, 400, "only image files are supported")
		return
	}

	if err := os.MkdirAll("uploads/system", 0o755); err != nil {
		httpx.Failed(c, 500, "failed to create upload directory")
		return
	}

	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	target := filepath.Join("uploads", "system", filename)
	if err := c.SaveUploadedFile(file, target); err != nil {
		httpx.Failed(c, 500, "failed to save uploaded file")
		return
	}

	httpx.Success(c, gin.H{
		"url": "/" + filepath.ToSlash(target),
	})
}

func (ctl *Controller) DownloadAssetHostTemplate(c *gin.Context) {
	file := excelize.NewFile()
	sheet := file.GetSheetName(0)
	headers := []string{"호스트 이름*", "SSH 주소*", "SSH 포트", "SSH 사용자*", "인증 Credential*", "연결 방식*", "접속 Gateway", "소속 Environment*", "Private IP", "Public IP", "Cloud Provider", "Region", "비고"}
	for idx, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(idx+1, 1)
		_ = file.SetCellValue(sheet, cell, header)
	}
	_ = file.SetCellValue(sheet, "A2", "web-01")
	_ = file.SetCellValue(sheet, "B2", "192.168.101.159")
	_ = file.SetCellValue(sheet, "C2", 22)
	_ = file.SetCellValue(sheet, "D2", "root")
	_ = file.SetCellValue(sheet, "E2", "default-ssh")
	_ = file.SetCellValue(sheet, "F2", "direct")
	_ = file.SetCellValue(sheet, "H2", "production")
	_ = file.SetCellValue(sheet, "I2", "192.168.101.159")
	_ = file.SetCellValue(sheet, "K2", "On-premises")
	_ = file.SetCellValue(sheet, "M2", "Excel Import 예시")
	_ = file.SetColWidth(sheet, "A", "M", 18)
	_ = file.SetColWidth(sheet, "M", "M", 30)
	_ = file.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
	_, err := file.NewSheet("작성 안내")
	if err == nil {
		_ = file.SetCellValue("작성 안내", "A1", "필드 설명")
		_ = file.SetCellValue("작성 안내", "A2", "* 표시 열은 필수입니다. Target Host Group은 Import 대화상자에서 선택합니다.")
		_ = file.SetCellValue("작성 안내", "A3", "연결 방식은 direct 또는 gateway만 지원합니다. gateway 사용 시 활성화된 접속 Gateway 이름이 필요합니다.")
		_ = file.SetCellValue("작성 안내", "A4", "인증 Credential과 접속 Gateway는 이름으로 일치시키며 플랫폼에 생성되어 활성화되어 있어야 합니다.")
		_ = file.SetCellValue("작성 안내", "A5", "Environment에는 production 또는 test와 같은 Code를 입력하고 SSH 주소에는 Private IP 사용을 권장합니다.")
		_ = file.SetColWidth("작성 안내", "A", "A", 100)
		file.SetActiveSheet(0)
	}
	buffer, err := file.WriteToBuffer()
	if err != nil {
		httpx.Failed(c, 500, "failed to generate template")
		return
	}
	c.Header("Content-Disposition", "attachment; filename=asset-host-template.xlsx")
	c.Data(200, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}

func mustAtoi(v string) int {
	n, _ := strconv.Atoi(v)
	return n
}

func safeExcelCell(row []string, index int) string {
	if index < len(row) {
		return strings.TrimSpace(row[index])
	}
	return ""
}

func BuildMenuTree(list []model.Menu) []gin.H {
	nodes := make(map[uint]gin.H, len(list))
	result := make([]gin.H, 0)
	for _, item := range list {
		node := gin.H{
			"id":         item.ID,
			"parentId":   item.ParentID,
			"menuName":   item.MenuName,
			"icon":       item.Icon,
			"value":      item.Value,
			"menuType":   item.MenuType,
			"url":        item.URL,
			"menuStatus": item.MenuStatus,
			"sort":       item.Sort,
			"createTime": item.CreatedAt,
			"children":   []gin.H{},
		}
		nodes[item.ID] = node
	}
	for _, item := range list {
		if item.ParentID == 0 {
			result = append(result, nodes[item.ID])
			continue
		}
		parent := nodes[item.ParentID]
		if parent == nil {
			continue
		}
		parentChildren := parent["children"].([]gin.H)
		parentChildren = append(parentChildren, nodes[item.ID])
		parent["children"] = parentChildren
		nodes[item.ParentID] = parent
	}
	return result
}
