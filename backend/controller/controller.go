package controller

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"ops-admin/backend/httpx"
	"ops-admin/backend/model"
	"ops-admin/backend/service"

	"github.com/gin-gonic/gin"
)

type Controller struct {
	service *service.Service
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
	data, err := ctl.service.Login(req, c.ClientIP(), c.Request.UserAgent(), runtime.GOOS)
	if err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, data)
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
		httpx.Failed(c, 400, "请选择要上传的文件")
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg":
	default:
		httpx.Failed(c, 400, "仅支持图片文件上传")
		return
	}

	if err := os.MkdirAll("uploads/system", 0o755); err != nil {
		httpx.Failed(c, 500, "创建上传目录失败")
		return
	}

	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	target := filepath.Join("uploads", "system", filename)
	if err := c.SaveUploadedFile(file, target); err != nil {
		httpx.Failed(c, 500, "保存文件失败")
		return
	}

	httpx.Success(c, gin.H{
		"url": "/" + filepath.ToSlash(target),
	})
}

func (ctl *Controller) GetSysAdminList(c *gin.Context) {
	pageNum, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	data, err := ctl.service.ListAdmins(pageNum, pageSize, c.Query("username"), c.Query("status"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) GetSysAdminInfo(c *gin.Context) {
	id := uint(mustAtoi(c.Query("id")))
	data, err := ctl.service.GetAdmin(id)
	if err != nil {
		httpx.Failed(c, 404, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) CreateSysAdmin(c *gin.Context) {
	var payload service.AdminPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid user payload")
		return
	}
	if err := ctl.service.CreateAdmin(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) UpdateSysAdmin(c *gin.Context) {
	var payload service.AdminPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid user payload")
		return
	}
	if err := ctl.service.UpdateAdmin(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) DeleteSysAdmin(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid delete payload")
		return
	}
	if err := ctl.service.DeleteAdmin(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) UpdateSysAdminStatus(c *gin.Context) {
	var payload service.AdminStatusPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid status payload")
		return
	}
	if err := ctl.service.UpdateAdminStatus(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) ResetSysAdminPassword(c *gin.Context) {
	var payload struct {
		ID       uint   `json:"id"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid password payload")
		return
	}
	if err := ctl.service.ResetAdminPassword(payload.ID, payload.Password); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) UpdatePersonal(c *gin.Context) {
	var payload service.AdminPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid profile payload")
		return
	}
	if err := ctl.service.UpdateProfile(c.GetUint("userID"), payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) UpdatePersonalPassword(c *gin.Context) {
	var payload struct {
		Password      string `json:"password"`
		NewPassword   string `json:"newPassword"`
		ResetPassword string `json:"resetPassword"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid password payload")
		return
	}
	if payload.NewPassword != payload.ResetPassword {
		httpx.Failed(c, 400, "new password confirmation mismatch")
		return
	}
	if err := ctl.service.UpdateProfilePassword(c.GetUint("userID"), payload.Password, payload.NewPassword); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) GetRoleList(c *gin.Context) {
	list, err := ctl.service.ListRoles()
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, map[string]any{"list": list, "total": len(list)})
}

func (ctl *Controller) QuerySysRoleVoList(c *gin.Context) {
	list, err := ctl.service.ListRoles()
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	vo := make([]gin.H, 0, len(list))
	for _, item := range list {
		vo = append(vo, gin.H{"id": item.ID, "roleName": item.RoleName})
	}
	httpx.Success(c, vo)
}

func (ctl *Controller) CreateRole(c *gin.Context) {
	var payload service.RolePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid role payload")
		return
	}
	if err := ctl.service.CreateRole(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) UpdateRole(c *gin.Context) {
	var payload service.RolePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid role payload")
		return
	}
	if err := ctl.service.UpdateRole(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) DeleteRole(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid delete payload")
		return
	}
	if err := ctl.service.DeleteRole(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) GetRoleInfo(c *gin.Context) {
	role, err := ctl.service.GetRole(uint(mustAtoi(c.Query("id"))))
	if err != nil {
		httpx.Failed(c, 404, err.Error())
		return
	}
	httpx.Success(c, role)
}

func (ctl *Controller) UpdateRoleStatus(c *gin.Context) {
	var payload service.RoleStatusPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid status payload")
		return
	}
	if err := ctl.service.UpdateRoleStatus(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) QueryRoleMenuIDList(c *gin.Context) {
	ids, err := ctl.service.RoleMenuIDs(uint(mustAtoi(c.Query("id"))))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	result := make([]gin.H, 0, len(ids))
	for _, id := range ids {
		result = append(result, gin.H{"id": id})
	}
	httpx.Success(c, result)
}

func (ctl *Controller) AssignPermissions(c *gin.Context) {
	var payload service.RoleMenuPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid permissions payload")
		return
	}
	if err := ctl.service.AssignRoleMenus(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) GetMenuList(c *gin.Context) {
	list, err := ctl.service.ListMenus()
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, list)
}

func (ctl *Controller) QuerySysMenuVoList(c *gin.Context) {
	list, err := ctl.service.ListMenus()
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	vo := make([]gin.H, 0, len(list))
	for _, item := range list {
		vo = append(vo, gin.H{"id": item.ID, "parentId": item.ParentID, "label": item.MenuName})
	}
	httpx.Success(c, vo)
}

func (ctl *Controller) CreateMenu(c *gin.Context) {
	var payload service.MenuPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid menu payload")
		return
	}
	if err := ctl.service.CreateMenu(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) UpdateMenu(c *gin.Context) {
	var payload service.MenuPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid menu payload")
		return
	}
	if err := ctl.service.UpdateMenu(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) DeleteMenu(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid delete payload")
		return
	}
	if err := ctl.service.DeleteMenu(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) GetMenuInfo(c *gin.Context) {
	menu, err := ctl.service.GetMenu(uint(mustAtoi(c.Query("id"))))
	if err != nil {
		httpx.Failed(c, 404, err.Error())
		return
	}
	httpx.Success(c, menu)
}

func (ctl *Controller) GetDeptList(c *gin.Context) {
	list, err := ctl.service.ListDepts()
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, list)
}

func (ctl *Controller) QuerySysDeptVoList(c *gin.Context) {
	list, err := ctl.service.ListDepts()
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	vo := make([]gin.H, 0, len(list))
	for _, item := range list {
		vo = append(vo, gin.H{"id": item.ID, "parentId": item.ParentID, "label": item.DeptName})
	}
	httpx.Success(c, vo)
}

func (ctl *Controller) CreateDept(c *gin.Context) {
	var payload service.DeptPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid department payload")
		return
	}
	if err := ctl.service.CreateDept(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) UpdateDept(c *gin.Context) {
	var payload service.DeptPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid department payload")
		return
	}
	if err := ctl.service.UpdateDept(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) DeleteDept(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid delete payload")
		return
	}
	if err := ctl.service.DeleteDept(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) GetDeptInfo(c *gin.Context) {
	dept, err := ctl.service.GetDept(uint(mustAtoi(c.Query("id"))))
	if err != nil {
		httpx.Failed(c, 404, err.Error())
		return
	}
	httpx.Success(c, dept)
}

func (ctl *Controller) GetDeptUsers(c *gin.Context) {
	deptID := uint(mustAtoi(c.Query("deptId")))
	users, err := ctl.service.DeptUsers(deptID)
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, users)
}

func (ctl *Controller) GetPostList(c *gin.Context) {
	list, err := ctl.service.ListPosts()
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, map[string]any{"list": list, "total": len(list)})
}

func (ctl *Controller) QuerySysPostVoList(c *gin.Context) {
	list, err := ctl.service.ListPosts()
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	vo := make([]gin.H, 0, len(list))
	for _, item := range list {
		vo = append(vo, gin.H{"id": item.ID, "postName": item.PostName})
	}
	httpx.Success(c, vo)
}

func (ctl *Controller) CreatePost(c *gin.Context) {
	var payload service.PostPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid post payload")
		return
	}
	if err := ctl.service.CreatePost(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) UpdatePost(c *gin.Context) {
	var payload service.PostPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid post payload")
		return
	}
	if err := ctl.service.UpdatePost(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) DeletePost(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid delete payload")
		return
	}
	if err := ctl.service.DeletePost(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) BatchDeletePost(c *gin.Context) {
	var payload service.BatchIDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid batch delete payload")
		return
	}
	if err := ctl.service.BatchDeletePosts(payload.IDs); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) GetPostInfo(c *gin.Context) {
	post, err := ctl.service.GetPost(uint(mustAtoi(c.Query("id"))))
	if err != nil {
		httpx.Failed(c, 404, err.Error())
		return
	}
	httpx.Success(c, post)
}

func (ctl *Controller) UpdatePostStatus(c *gin.Context) {
	var payload service.PostStatusPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid status payload")
		return
	}
	if err := ctl.service.UpdatePostStatus(payload); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) GetLoginLogList(c *gin.Context) {
	pageNum, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	data, err := ctl.service.ListLoginLogs(pageNum, pageSize, c.Query("username"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) DeleteLoginLog(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid delete payload")
		return
	}
	if err := ctl.service.DeleteLoginLog(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) BatchDeleteLoginLog(c *gin.Context) {
	var payload service.BatchIDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid batch delete payload")
		return
	}
	if err := ctl.service.BatchDeleteLoginLogs(payload.IDs); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) CleanLoginLog(c *gin.Context) {
	if err := ctl.service.CleanLoginLogs(); err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) GetOperationLogList(c *gin.Context) {
	pageNum, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	data, err := ctl.service.ListOperationLogs(pageNum, pageSize, c.Query("username"))
	if err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, data)
}

func (ctl *Controller) DeleteOperationLog(c *gin.Context) {
	var payload service.IDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid delete payload")
		return
	}
	if err := ctl.service.DeleteOperationLog(payload.ID); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) BatchDeleteOperationLog(c *gin.Context) {
	var payload service.BatchIDPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Failed(c, 400, "invalid batch delete payload")
		return
	}
	if err := ctl.service.BatchDeleteOperationLogs(payload.IDs); err != nil {
		httpx.Failed(c, 400, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (ctl *Controller) CleanOperationLog(c *gin.Context) {
	if err := ctl.service.CleanOperationLogs(); err != nil {
		httpx.Failed(c, 500, err.Error())
		return
	}
	httpx.Success(c, true)
}

func mustAtoi(v string) int {
	n, _ := strconv.Atoi(v)
	return n
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
