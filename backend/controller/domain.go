package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"ops-admin/backend/httpx"
	"ops-admin/backend/internal/domain/provider"
	"ops-admin/backend/service"
)

func dnsActor(c *gin.Context) service.DNSAuditActor {
	return service.DNSAuditActor{AdminID: c.GetUint("userID"), Username: c.GetString("username"), IP: c.ClientIP()}
}

func (ctl *Controller) ListPublicDNSAccounts(c *gin.Context) {
	pageNum, pageSize := mustAtoi(c.DefaultQuery("pageNum", "1")), mustAtoi(c.DefaultQuery("pageSize", "10"))
	data, err := ctl.service.ListPublicDNSAccounts(pageNum, pageSize, c.Query("keyword"), c.Query("provider"))
	respond(c, data, err)
}
func (ctl *Controller) PublicDNSAccountOptions(c *gin.Context) {
	data, err := ctl.service.PublicDNSAccountOptions()
	respond(c, data, err)
}
func (ctl *Controller) GetPublicDNSAccount(c *gin.Context) {
	data, err := ctl.service.GetPublicDNSAccount(uint(mustAtoi(c.Query("id"))))
	respond(c, data, err)
}
func (ctl *Controller) SavePublicDNSAccount(c *gin.Context) {
	var payload service.PublicDNSAccountPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid DNS account payload")
		return
	}
	err := ctl.service.SavePublicDNSAccount(payload, dnsActor(c))
	respond(c, gin.H{"saved": err == nil}, err)
}
func (ctl *Controller) DeletePublicDNSAccount(c *gin.Context) {
	var payload struct {
		ID uint `json:"id"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid DNS account payload")
		return
	}
	err := ctl.service.DeletePublicDNSAccount(payload.ID, dnsActor(c))
	respond(c, gin.H{"deleted": err == nil}, err)
}
func (ctl *Controller) TestPublicDNSAccount(c *gin.Context) {
	var payload struct {
		ID uint `json:"id"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid DNS account payload")
		return
	}
	err := ctl.service.TestPublicDNSAccount(payload.ID)
	respond(c, gin.H{"connected": err == nil}, err)
}
func (ctl *Controller) SyncPublicDomains(c *gin.Context) {
	var payload struct {
		AccountID uint `json:"accountId"`
	}
	_ = c.ShouldBindJSON(&payload)
	count, err := ctl.service.SyncPublicDomains(payload.AccountID)
	respond(c, gin.H{"synced": count}, err)
}
func (ctl *Controller) ListPublicDomains(c *gin.Context) {
	pageNum, pageSize := mustAtoi(c.DefaultQuery("pageNum", "1")), mustAtoi(c.DefaultQuery("pageSize", "10"))
	accountID, _ := strconv.ParseUint(c.Query("accountId"), 10, 64)
	data, err := ctl.service.ListPublicDomains(pageNum, pageSize, c.Query("keyword"), c.Query("provider"), uint(accountID))
	respond(c, data, err)
}
func (ctl *Controller) ListPublicDNSRecords(c *gin.Context) {
	accountID, _ := strconv.ParseUint(c.Query("accountId"), 10, 64)
	data, err := ctl.service.ListPublicRecords(uint(accountID), c.Query("domain"))
	respond(c, data, err)
}
func (ctl *Controller) MutatePublicDNSRecord(c *gin.Context) {
	var payload struct {
		AccountID uint                   `json:"accountId"`
		Action    string                 `json:"action"`
		Record    provider.RecordRequest `json:"record"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid DNS record payload")
		return
	}
	err := ctl.service.MutatePublicRecord(strings.ToLower(payload.Action), payload.AccountID, payload.Record, dnsActor(c))
	respond(c, gin.H{"saved": err == nil}, err)
}
func (ctl *Controller) BatchPublicDNSRecords(c *gin.Context) {
	var payload service.PublicBatchPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid DNS batch payload")
		return
	}
	httpx.Success(c, ctl.service.BatchPublicRecords(payload, dnsActor(c)))
}

func (ctl *Controller) GetInternalDNSSettings(c *gin.Context) {
	data, err := ctl.service.GetInternalDNSSettings()
	respond(c, data, err)
}
func (ctl *Controller) SaveInternalDNSSettings(c *gin.Context) {
	var payload service.InternalDNSSettingsPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid internal DNS settings")
		return
	}
	err := ctl.service.SaveInternalDNSSettings(payload, dnsActor(c))
	respond(c, gin.H{"saved": err == nil}, err)
}
func (ctl *Controller) ListInternalDNSZones(c *gin.Context) {
	data, err := ctl.service.ListInternalZones(c.Query("keyword"))
	respond(c, data, err)
}
func (ctl *Controller) SaveInternalDNSZone(c *gin.Context) {
	var payload service.InternalZonePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid DNS zone payload")
		return
	}
	err := ctl.service.SaveInternalZone(payload, dnsActor(c))
	respond(c, gin.H{"saved": err == nil}, err)
}
func (ctl *Controller) DeleteInternalDNSZone(c *gin.Context) {
	var payload struct {
		ID uint `json:"id"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid DNS zone payload")
		return
	}
	err := ctl.service.DeleteInternalZone(payload.ID, dnsActor(c))
	respond(c, gin.H{"deleted": err == nil}, err)
}
func (ctl *Controller) ListInternalDNSRecords(c *gin.Context) {
	zoneID, _ := strconv.ParseUint(c.Query("zoneId"), 10, 64)
	data, err := ctl.service.ListInternalRecords(uint(zoneID), c.Query("keyword"))
	respond(c, data, err)
}
func (ctl *Controller) SaveInternalDNSRecord(c *gin.Context) {
	var payload service.InternalRecordPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid DNS record payload")
		return
	}
	err := ctl.service.SaveInternalRecord(payload, dnsActor(c))
	respond(c, gin.H{"saved": err == nil}, err)
}
func (ctl *Controller) DeleteInternalDNSRecord(c *gin.Context) {
	var payload struct {
		ID uint `json:"id"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid DNS record payload")
		return
	}
	err := ctl.service.DeleteInternalRecord(payload.ID, dnsActor(c))
	respond(c, gin.H{"deleted": err == nil}, err)
}
func (ctl *Controller) BatchInternalDNSRecords(c *gin.Context) {
	var payload service.InternalRecordBatchPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid internal DNS batch payload")
		return
	}
	affected, err := ctl.service.BatchInternalRecords(payload, dnsActor(c))
	respond(c, gin.H{"affected": affected}, err)
}
func (ctl *Controller) TestDNSResolution(c *gin.Context) {
	var payload struct {
		Domain string `json:"domain"`
		Type   string `json:"type"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid DNS query payload")
		return
	}
	data, err := ctl.service.TestDNSResolution(payload.Domain, payload.Type)
	respond(c, data, err)
}
func (ctl *Controller) ListDNSAuditLogs(c *gin.Context) {
	pageNum := mustAtoi(c.DefaultQuery("pageNum", "1"))
	pageSize := mustAtoi(c.DefaultQuery("pageSize", "20"))
	data, err := ctl.service.ListDNSAuditLogs(pageNum, pageSize)
	respond(c, data, err)
}

func (ctl *Controller) ListSSLCertificates(c *gin.Context) {
	pageNum, pageSize := mustAtoi(c.DefaultQuery("pageNum", "1")), mustAtoi(c.DefaultQuery("pageSize", "10"))
	accountID, _ := strconv.ParseUint(c.Query("accountId"), 10, 64)
	data, err := ctl.service.ListSSLCertificates(pageNum, pageSize, c.Query("keyword"), c.Query("status"), c.Query("source"), uint(accountID))
	respond(c, data, err)
}

func (ctl *Controller) GetSSLCertificate(c *gin.Context) {
	data, err := ctl.service.GetSSLCertificate(uint(mustAtoi(c.Query("id"))))
	respond(c, data, err)
}

func (ctl *Controller) SSLCertificateDomainOptions(c *gin.Context) {
	data, err := ctl.service.SSLCertificateDomainOptions()
	respond(c, data, err)
}

func (ctl *Controller) SyncSSLCertificates(c *gin.Context) {
	var payload struct {
		AccountID uint `json:"accountId"`
	}
	_ = c.ShouldBindJSON(&payload)
	ids, err := ctl.service.QueueCertificateCloudSync(payload.AccountID, dnsActor(c))
	respond(c, gin.H{"taskIds": ids}, err)
}

func (ctl *Controller) UploadSSLCertificate(c *gin.Context) {
	var payload service.SSLCertificateUploadPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid SSL certificate upload payload")
		return
	}
	id, err := ctl.service.UploadSSLCertificate(payload, dnsActor(c))
	respond(c, gin.H{"id": id}, err)
}

func (ctl *Controller) ApplySSLCertificate(c *gin.Context) {
	var payload service.SSLCertificateApplyPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid SSL certificate application payload")
		return
	}
	id, taskID, err := ctl.service.CreateSSLCertificateApplication(payload, dnsActor(c))
	respond(c, gin.H{"id": id, "taskId": taskID}, err)
}

func (ctl *Controller) RenewSSLCertificate(c *gin.Context) {
	var payload struct {
		ID uint `json:"id"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid SSL certificate renew payload")
		return
	}
	taskID, err := ctl.service.QueueCertificateTask(payload.ID, "RENEW", dnsActor(c))
	respond(c, gin.H{"taskId": taskID}, err)
}

func (ctl *Controller) ResyncSSLCertificate(c *gin.Context) {
	var payload struct {
		ID uint `json:"id"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid SSL certificate sync payload")
		return
	}
	taskID, err := ctl.service.QueueCertificateTask(payload.ID, "SYNC", dnsActor(c))
	respond(c, gin.H{"taskId": taskID}, err)
}

func (ctl *Controller) UpdateSSLCertificateRenewSettings(c *gin.Context) {
	var payload service.SSLCertificateRenewSettingPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid SSL renewal settings")
		return
	}
	err := ctl.service.UpdateSSLCertificateRenewSettings(payload, dnsActor(c))
	respond(c, gin.H{"saved": err == nil}, err)
}

func (ctl *Controller) DeleteSSLCertificate(c *gin.Context) {
	var payload service.SSLCertificateDeletePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid SSL certificate delete payload")
		return
	}
	taskID, err := ctl.service.DeleteSSLCertificate(payload, dnsActor(c))
	respond(c, gin.H{"taskId": taskID, "deleted": err == nil && taskID == 0}, err)
}

func (ctl *Controller) ListSSLCertificateTasks(c *gin.Context) {
	certificateID, _ := strconv.ParseUint(c.Query("certificateId"), 10, 64)
	data, err := ctl.service.ListSSLCertificateTasks(uint(certificateID), mustAtoi(c.DefaultQuery("limit", "30")))
	respond(c, data, err)
}

func (ctl *Controller) ListSSLCertificateAudits(c *gin.Context) {
	data, err := ctl.service.ListSSLCertificateAudits(mustAtoi(c.DefaultQuery("limit", "100")))
	respond(c, data, err)
}

func (ctl *Controller) DownloadSSLCertificate(c *gin.Context) {
	ctl.downloadSSLCertificate(c, false)
}

func (ctl *Controller) DownloadSSLCertificatePrivate(c *gin.Context) {
	ctl.downloadSSLCertificate(c, true)
}

func (ctl *Controller) downloadSSLCertificate(c *gin.Context, private bool) {
	id := uint(mustAtoi(c.Query("id")))
	kind := strings.ToLower(c.DefaultQuery("type", "certificate"))
	if private && kind != "private-key" && kind != "zip" {
		httpx.Failed(c, 400, "敏感下载接口仅支持 private-key 或 zip")
		return
	}
	if !private && kind != "certificate" && kind != "chain" {
		httpx.Failed(c, 400, "普通下载接口仅支持 certificate 或 chain")
		return
	}
	data, filename, contentType, err := ctl.service.DownloadSSLCertificate(id, kind)
	ctl.service.AuditSSLCertificateDownload(id, kind, dnsActor(c), err)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, contentType, data)
}

func respond(c *gin.Context, data any, err error) {
	if err != nil {
		httpx.Failed(c, http.StatusBadRequest, err.Error())
		return
	}
	httpx.Success(c, data)
}
