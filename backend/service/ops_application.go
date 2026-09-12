package service

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"ops-admin/backend/model"
)

type OpsApplicationPayload struct {
	ID               uint                                      `json:"id"`
	Name             string                                    `json:"name"`
	Code             string                                    `json:"code"`
	ServiceType      string                                    `json:"serviceType"`
	RepoType         string                                    `json:"repoType"`
	RepoURL          string                                    `json:"repoUrl"`
	RepoCredentialID uint                                      `json:"repoCredentialId"`
	Branch           string                                    `json:"branch"`
	Workspace        string                                    `json:"workspace"`
	BuildScript      string                                    `json:"buildScript"`
	DeployScript     string                                    `json:"deployScript"`
	Env              string                                    `json:"env"`
	Status           int                                       `json:"status"`
	Description      string                                    `json:"description"`
	Bindings         []OpsApplicationEnvironmentBindingPayload `json:"bindings"`
}

type OpsApplicationEnvironmentBindingPayload struct {
	Env                 string `json:"env"`
	HostGroupID         uint   `json:"hostGroupId"`
	Namespace           string `json:"namespace"`
	WorkloadType        string `json:"workloadType"`
	WorkloadName        string `json:"workloadName"`
	DatabaseID          uint   `json:"databaseId"`
	MonitorDatasourceID uint   `json:"monitorDatasourceId"`
	GatewayID           uint   `json:"gatewayId"`
}

func normalizeOpsRepoType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "svn":
		return "svn"
	default:
		return "git"
	}
}

func normalizeOpsSVNRevision(value string) (string, error) {
	revision := strings.ToUpper(strings.TrimSpace(value))
	if revision == "" || revision == "HEAD" {
		return "HEAD", nil
	}
	for _, char := range revision {
		if char < '0' || char > '9' {
			return "", errors.New("SVN revision supports only HEAD or a numeric revision")
		}
	}
	return revision, nil
}

func normalizeOpsAppStatus(value int) int {
	if value == 2 {
		return 2
	}
	return 1
}

func normalizeOpsBuildTimeout(value int) int {
	if value < 60 {
		return 60
	}
	if value > 7200 {
		return 7200
	}
	return value
}

func (s *Service) ListOpsApplications(pageNum, pageSize int, keyword, repoType, status, serviceType, env string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(&model.OpsApplication{})
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR code LIKE ? OR repo_url LIKE ? OR description LIKE ?", like, like, like, like)
	}
	if repoType = strings.ToLower(strings.TrimSpace(repoType)); repoType != "" {
		query = query.Where("repo_type = ?", normalizeOpsRepoType(repoType))
	}
	if status = strings.TrimSpace(status); status != "" {
		query = query.Where("status = ?", status)
	}
	if serviceType = strings.TrimSpace(serviceType); serviceType != "" {
		query = query.Where("service_type = ?", serviceType)
	}
	if env = normalizeEnvCode(env); env != "" {
		bindingAppIDs := s.db.Model(&model.OpsApplicationEnvironmentBinding{}).Select("app_id").Where("env = ? AND status = ?", env, 1)
		query = query.Where("env = ? OR id IN (?)", env, bindingAppIDs)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.OpsApplication
	if err := query.Order("id DESC").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
}

func (s *Service) ListOpsApplicationOptions() ([]map[string]any, error) {
	var list []model.OpsApplication
	if err := s.db.Where("status = ?", 1).Order("name ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	options := make([]map[string]any, 0, len(list))
	for _, item := range list {
		options = append(options, map[string]any{
			"id": item.ID, "name": item.Name, "code": item.Code, "repoType": item.RepoType,
			"repoUrl": item.RepoURL, "branch": item.Branch, "workspace": item.Workspace, "env": item.Env, "serviceType": item.ServiceType,
			"repoCredentialId": item.RepoCredentialID,
		})
	}
	return options, nil
}

func (s *Service) GetOpsApplication(id uint) (*model.OpsApplication, error) {
	var item model.OpsApplication
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) SaveOpsApplication(payload OpsApplicationPayload) error {
	item := model.OpsApplication{
		Name:             Trimmed(payload.Name),
		Code:             Trimmed(payload.Code),
		ServiceType:      Trimmed(payload.ServiceType),
		RepoType:         normalizeOpsRepoType(payload.RepoType),
		RepoURL:          Trimmed(payload.RepoURL),
		RepoCredentialID: payload.RepoCredentialID,
		Branch:           Trimmed(payload.Branch),
		BuildScript:      strings.TrimSpace(payload.BuildScript),
		DeployScript:     strings.TrimSpace(payload.DeployScript),
		Env:              Trimmed(payload.Env),
		Status:           normalizeOpsAppStatus(payload.Status),
		Description:      Trimmed(payload.Description),
	}
	if item.Name == "" {
		return errors.New("application name is required")
	}
	if item.Code == "" {
		return errors.New("application code is required")
	}
	if item.RepoURL == "" {
		return errors.New("repository URL is required")
	}
	if item.ServiceType == "" {
		item.ServiceType = "Backend Service"
	}
	if item.RepoType == "svn" {
		var err error
		item.Branch, err = normalizeOpsSVNRevision(item.Branch)
		if err != nil {
			return err
		}
	} else if item.Branch == "" {
		item.Branch = "master"
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		appID := payload.ID
		if appID == 0 {
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
			appID = item.ID
		} else if err := tx.Model(&model.OpsApplication{}).Where("id = ?", appID).Updates(map[string]any{
			"name": item.Name, "code": item.Code, "service_type": item.ServiceType, "repo_type": item.RepoType,
			"repo_url": item.RepoURL, "repo_credential_id": item.RepoCredentialID, "branch": item.Branch,
			"build_script": item.BuildScript, "deploy_script": item.DeployScript, "env": item.Env,
			"status": item.Status, "description": item.Description,
		}).Error; err != nil {
			return err
		}
		if payload.Bindings == nil {
			return nil
		}
		if err := tx.Where("app_id = ?", appID).Delete(&model.OpsApplicationEnvironmentBinding{}).Error; err != nil {
			return err
		}
		seen := map[string]struct{}{}
		for _, binding := range payload.Bindings {
			env := normalizeEnvCode(binding.Env)
			if env == "" {
				continue
			}
			if _, ok := seen[env]; ok {
				return fmt.Errorf("environment %s can be bound only once", env)
			}
			seen[env] = struct{}{}
			row := model.OpsApplicationEnvironmentBinding{
				AppID: appID, Env: env, HostGroupID: binding.HostGroupID,
				Namespace: Trimmed(binding.Namespace), WorkloadType: strings.ToLower(Trimmed(binding.WorkloadType)),
				WorkloadName: Trimmed(binding.WorkloadName), DatabaseID: binding.DatabaseID,
				MonitorDatasourceID: binding.MonitorDatasourceID, GatewayID: binding.GatewayID, Status: 1,
			}
			if row.WorkloadType == "" {
				row.WorkloadType = "deployment"
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) ListOpsApplicationEnvironmentBindings(appID uint) ([]model.OpsApplicationEnvironmentBinding, error) {
	var list []model.OpsApplicationEnvironmentBinding
	err := s.db.Where("app_id = ?", appID).Order("env ASC").Find(&list).Error
	return list, err
}

func (s *Service) DeleteOpsApplication(id uint) error {
	var count int64
	if err := s.db.Model(&model.OpsAppBuildTask{}).Where("app_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("the application still has build tasks; delete them first")
	}
	if err := s.db.Model(&model.OpsAppPipeline{}).Where("app_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("the application still has CI/CD pipelines; delete them first")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("app_id = ?", id).Delete(&model.OpsApplicationEnvironmentBinding{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.OpsApplication{}, id).Error
	})
}
