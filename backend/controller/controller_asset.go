package controller

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"ops-admin/backend/httpx"
	"ops-admin/backend/service"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/xuri/excelize/v2"
)

func (ctl *Controller) GetAssetHostList(c *gin.Context) {
	pageNum, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	data, err := ctl.service.ListAssetHosts(pageNum, pageSize, c.Query("keyword"), uint(mustAtoi(c.Query("groupId"))), c.Query("status"), c.Query("environment"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetAssetOverview(c *gin.Context) {
	data, err := ctl.service.GetAssetOverview()
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetAssetChangeLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	data, err := ctl.service.ListAssetChangeLogs(c.Query("resourceType"), uint(mustAtoi(c.Query("resourceId"))), limit)
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetAssetHostInfo(c *gin.Context) {
	data, err := ctl.service.GetAssetHost(uint(mustAtoi(c.Query("id"))))
	if err != nil {
		httpx.Failed(c, 404, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetAssetHostMetrics(c *gin.Context) {
	data, err := ctl.service.GetAssetHostMetrics(uint(mustAtoi(c.Query("id"))), c.DefaultQuery("range", "1h"), c.Query("start"), c.Query("end"))
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) CreateAssetHost(c *gin.Context) {
	var payload service.AssetHostPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid host payload")
		return
	}
	payload.Operator = c.GetString("username")
	if err := ctl.service.CreateAssetHost(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) UpdateAssetHost(c *gin.Context) {
	var payload service.AssetHostPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid host payload")
		return
	}
	payload.Operator = c.GetString("username")
	if err := ctl.service.UpdateAssetHost(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) ImportAssetHosts(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		httpx.Failed(c, 400, "please upload excel file")
		return
	}
	groupID := uint(mustAtoi(c.PostForm("groupId")))
	if groupID == 0 {
		httpx.Failed(c, 400, "groupId is required")
		return
	}
	if err := os.MkdirAll("uploads/temp", 0o755); err != nil {
		httpx.Failed(c, 500, "failed to create temp dir")
		return
	}
	tempPath := filepath.Join("uploads", "temp", fmt.Sprintf("asset-host-import-%d%s", time.Now().UnixNano(), filepath.Ext(file.Filename)))
	if err := c.SaveUploadedFile(file, tempPath); err != nil {
		httpx.Failed(c, 500, "failed to save excel file")
		return
	}
	defer os.Remove(tempPath)

	workbook, err := excelize.OpenFile(tempPath)
	if err != nil {
		httpx.Failed(c, 400, "failed to parse excel file")
		return
	}
	defer workbook.Close()

	sheets := workbook.GetSheetList()
	if len(sheets) == 0 {
		httpx.Failed(c, 400, "excel sheet is empty")
		return
	}
	rows, err := workbook.GetRows(sheets[0])
	if err != nil {
		httpx.Failed(c, 400, "failed to read excel rows")
		return
	}
	if len(rows) == 0 || len(rows[0]) < 8 {
		httpx.Failed(c, 400, "the Excel template format has changed; download the latest template before importing")
		return
	}

	importRows := make([]service.AssetHostImportRow, 0)
	for index, row := range rows {
		if index == 0 {
			continue
		}
		if len(row) < 8 {
			continue
		}
		importRows = append(importRows, service.AssetHostImportRow{
			HostName:       strings.TrimSpace(row[0]),
			SSHIP:          strings.TrimSpace(row[1]),
			SSHPort:        mustAtoi(strings.TrimSpace(row[2])),
			SSHUser:        strings.TrimSpace(row[3]),
			CredentialName: strings.TrimSpace(row[4]),
			ConnectionMode: strings.TrimSpace(row[5]),
			GatewayName:    safeExcelCell(row, 6),
			Environment:    safeExcelCell(row, 7),
			PrivateIP:      safeExcelCell(row, 8),
			PublicIP:       safeExcelCell(row, 9),
			Provider:       safeExcelCell(row, 10),
			Region:         safeExcelCell(row, 11),
			Description:    safeExcelCell(row, 12),
		})
	}

	data, err := ctl.service.ImportAssetHosts(groupID, importRows)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) SyncAssetHostsFromCloud(c *gin.Context) {
	var payload service.AssetCloudSyncPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid cloud sync payload")
		return
	}
	data, err := ctl.service.SyncAssetHostsFromCloud(payload)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) SyncAssetHost(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid sync payload")
		return
	}
	data, err := ctl.service.SyncAssetHost(payload.ID)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) BatchSyncAssetHosts(c *gin.Context) {
	var payload service.BatchIDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid batch sync payload")
		return
	}
	data, err := ctl.service.BatchSyncAssetHosts(payload.IDs)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) AssetTerminalWS(c *gin.Context) {
	// The canonical binding key is computed before the auth gate: minting and
	// consumption share the same resource-id normalization (Q1).
	bindingKey, bindErr := service.CanonicalConsoleResourceID(service.ConsoleResourceAssetHost, c.Query("hostId"))
	if bindErr != nil {
		httpx.Failed(c, 400, "invalid hostId")
		return
	}
	hostID := uint(mustAtoi(bindingKey))
	rows := mustAtoi(c.DefaultQuery("rows", "30"))
	cols := mustAtoi(c.DefaultQuery("cols", "120"))

	if !ctl.consumeTerminalTicket(c, service.ConsoleProtocolAssetTerminal, service.ConsoleResourceAssetHost, bindingKey) {
		return
	}

	terminal, err := ctl.service.OpenAssetTerminal(hostID, rows, cols)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	defer terminal.Session.Close()
	defer terminal.Client.Close()

	conn, err := terminalUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	var writeMu sync.Mutex
	writePipe := func(reader io.Reader) {
		buf := make([]byte, 4096)
		for {
			n, err := reader.Read(buf)
			if n > 0 {
				writeMu.Lock()
				_ = conn.WriteMessage(websocket.TextMessage, buf[:n])
				writeMu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}
	go writePipe(terminal.Stdout)
	go writePipe(terminal.Stderr)
	go func() {
		_ = terminal.Session.Wait()
		_ = conn.Close()
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if _, err := terminal.Stdin.Write(message); err != nil {
			return
		}
	}
}

func (ctl *Controller) DeleteAssetHost(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid delete payload")
		return
	}
	if err := ctl.service.DeleteAssetHost(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) BatchDeleteAssetHosts(c *gin.Context) {
	var payload service.BatchIDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid batch delete payload")
		return
	}
	if err := ctl.service.BatchDeleteAssetHosts(payload.IDs); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) RemoveAssetHostsFromGroup(c *gin.Context) {
	var payload service.AssetHostGroupMemberPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid remove group member payload")
		return
	}
	hostIDs := payload.HostIDs
	if payload.HostID > 0 {
		hostIDs = append(hostIDs, payload.HostID)
	}
	if err := ctl.service.RemoveAssetHostsFromGroup(payload.GroupID, hostIDs); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) BatchReplaceAssetHostCredential(c *gin.Context) {
	var payload service.AssetHostBatchCredentialPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid batch credential payload")
		return
	}
	if err := ctl.service.BatchReplaceAssetHostCredential(payload.IDs, payload.CredentialID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) GetAssetHostGroupList(c *gin.Context) {
	data, err := ctl.service.ListAssetHostGroups(c.Query("keyword"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetAssetHostGroupInfo(c *gin.Context) {
	data, err := ctl.service.GetAssetHostGroup(uint(mustAtoi(c.Query("id"))))
	if err != nil {
		httpx.Failed(c, 404, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) CreateAssetHostGroup(c *gin.Context) {
	var payload service.AssetHostGroupPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid host group payload")
		return
	}
	if err := ctl.service.CreateAssetHostGroup(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) UpdateAssetHostGroup(c *gin.Context) {
	var payload service.AssetHostGroupPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid host group payload")
		return
	}
	if err := ctl.service.UpdateAssetHostGroup(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) DeleteAssetHostGroup(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid delete payload")
		return
	}
	if err := ctl.service.DeleteAssetHostGroup(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) GetAssetCredentialList(c *gin.Context) {
	pageNum, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	data, err := ctl.service.ListAssetCredentials(pageNum, pageSize, c.Query("keyword"), c.Query("authType"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetAssetCredentialOptions(c *gin.Context) {
	data, err := ctl.service.ListAssetCredentialOptions()
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetAssetCredentialInfo(c *gin.Context) {
	data, err := ctl.service.GetAssetCredential(uint(mustAtoi(c.Query("id"))))
	if err != nil {
		httpx.Failed(c, 404, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) CreateAssetCredential(c *gin.Context) {
	var payload service.AssetCredentialPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid credential payload")
		return
	}
	if err := ctl.service.CreateAssetCredential(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) UpdateAssetCredential(c *gin.Context) {
	var payload service.AssetCredentialPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid credential payload")
		return
	}
	if err := ctl.service.UpdateAssetCredential(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) DeleteAssetCredential(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid delete payload")
		return
	}
	if err := ctl.service.DeleteAssetCredential(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) GetAssetCloudAccountList(c *gin.Context) {
	pageNum, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	data, err := ctl.service.ListAssetCloudAccounts(pageNum, pageSize, c.Query("keyword"), c.Query("provider"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetAssetCloudAccountOptions(c *gin.Context) {
	data, err := ctl.service.ListAssetCloudAccountOptions()
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetAssetCloudAccountInfo(c *gin.Context) {
	data, err := ctl.service.GetAssetCloudAccount(uint(mustAtoi(c.Query("id"))))
	if err != nil {
		httpx.Failed(c, 404, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) CreateAssetCloudAccount(c *gin.Context) {
	var payload service.AssetCloudAccountPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid cloud account payload")
		return
	}
	if err := ctl.service.CreateAssetCloudAccount(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) UpdateAssetCloudAccount(c *gin.Context) {
	var payload service.AssetCloudAccountPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid cloud account payload")
		return
	}
	if err := ctl.service.UpdateAssetCloudAccount(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) DeleteAssetCloudAccount(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid delete payload")
		return
	}
	if err := ctl.service.DeleteAssetCloudAccount(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}
