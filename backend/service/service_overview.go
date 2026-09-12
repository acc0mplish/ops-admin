package service

import (
	"errors"
	"sort"

	infraModel "ops-admin/backend/internal/infra/model"
	"ops-admin/backend/model"
)

func (s *Service) GetAssetOverview() (map[string]any, error) {
	var (
		hostTotal           int64
		hostOnline          int64
		hostOffline         int64
		hostAuthFailed      int64
		groupTotal          int64
		credentialTotal     int64
		credentialEnabled   int64
		cloudAccountTotal   int64
		cloudAccountEnabled int64
		databaseTotal       int64
		databaseHealthy     int64
		k8sClusterTotal     int64
		k8sClusterOnline    int64
		k8sNodeTotal        int64
		incompleteHosts     int64
		incompleteDatabases int64
		incompleteClusters  int64
	)

	if err := s.db.Model(&model.AssetHost{}).Count(&hostTotal).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.AssetHost{}).Where("alive_status = ?", 1).Count(&hostOnline).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.AssetHost{}).Where("alive_status = ?", 2).Count(&hostOffline).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.AssetHost{}).Where("auth_status = ?", 2).Count(&hostAuthFailed).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.AssetHostGroup{}).Count(&groupTotal).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.AssetCredential{}).Count(&credentialTotal).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.AssetCredential{}).Where("status = ?", 1).Count(&credentialEnabled).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.AssetCloudAccount{}).Count(&cloudAccountTotal).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.AssetCloudAccount{}).Where("status = ?", 1).Count(&cloudAccountEnabled).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.AssetDatabase{}).Count(&databaseTotal).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.AssetDatabase{}).Where("connect_status = ?", 1).Count(&databaseHealthy).Error; err != nil {
		return nil, err
	}
	// Kubernetes cluster stats resolve through the V2 connection source
	// (I-a S6 — the k8s_cluster aggregates are retired; §J5). "online" maps
	// to the connection status the V2 chains record ("active").
	k8sConns, err := s.k8sClusterConnections()
	if err != nil {
		return nil, err
	}
	k8sClusterTotal = int64(len(k8sConns))
	for i := range k8sConns {
		if k8sConns[i].Status == "active" {
			k8sClusterOnline++
		}
		if nodeCount, ok := k8sConfigUintValue(&k8sConns[i], "node_count"); ok {
			k8sNodeTotal += int64(nodeCount)
		}
		if k8sConfigString(&k8sConns[i], "env") == "" {
			incompleteClusters++
		}
	}
	if err := s.db.Model(&model.AssetHost{}).Where("TRIM(COALESCE(environment, '')) = ''").Count(&incompleteHosts).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.AssetDatabase{}).Where("TRIM(COALESCE(env, '')) = ''").Count(&incompleteDatabases).Error; err != nil {
		return nil, err
	}

	type distributionRow struct {
		Name  string `json:"name" gorm:"column:name"`
		Count int64  `json:"count" gorm:"column:count"`
	}

	var providerRows []distributionRow
	if err := s.db.Model(&model.AssetHost{}).
		Select("COALESCE(NULLIF(provider, ''), 'On-premises') as name, COUNT(*) as count").
		Group("COALESCE(NULLIF(provider, ''), 'On-premises')").
		Order("count DESC").
		Limit(8).
		Scan(&providerRows).Error; err != nil {
		return nil, err
	}

	var environmentRows []distributionRow
	if err := s.db.Model(&model.AssetHost{}).
		Where("NULLIF(environment, '') IS NOT NULL").
		Select("environment as name, COUNT(*) as count").
		Group("environment").
		Order("count DESC").
		Limit(8).
		Scan(&environmentRows).Error; err != nil {
		return nil, err
	}

	var groups []model.AssetHostGroup
	if err := s.db.Order("sort ASC, updated_at DESC").Find(&groups).Error; err != nil {
		return nil, err
	}
	countMap, err := s.assetHostGroupHostCountMap()
	if err != nil {
		return nil, err
	}

	groupItems := make([]map[string]any, 0, len(groups))
	for _, item := range groups {
		groupItems = append(groupItems, map[string]any{
			"id":        item.ID,
			"name":      item.Name,
			"code":      item.Code,
			"status":    item.Status,
			"hostCount": countMap[item.ID],
			"updatedAt": item.UpdatedAt,
		})
	}
	sort.Slice(groupItems, func(i, j int) bool {
		return groupItems[i]["hostCount"].(int64) > groupItems[j]["hostCount"].(int64)
	})
	if len(groupItems) > 6 {
		groupItems = groupItems[:6]
	}

	var recentHosts []model.AssetHost
	if err := s.db.Preload("Group").Preload("HostGroups").Preload("Credential").Preload("CloudAccount").
		Order("updated_at DESC").
		Limit(6).
		Find(&recentHosts).Error; err != nil {
		return nil, err
	}
	ensureHostGroupFallbackList(recentHosts)

	hostItems := make([]map[string]any, 0, len(recentHosts))
	for _, item := range recentHosts {
		groupNames := make([]string, 0, len(item.HostGroups))
		for _, group := range item.HostGroups {
			groupNames = append(groupNames, group.Name)
		}
		hostItems = append(hostItems, map[string]any{
			"id":          item.ID,
			"hostName":    item.HostName,
			"sshIp":       item.SSHIP,
			"publicIp":    item.PublicIP,
			"privateIp":   item.PrivateIP,
			"provider":    item.Provider,
			"environment": item.Environment,
			"aliveStatus": item.AliveStatus,
			"authStatus":  item.AuthStatus,
			"groupNames":  groupNames,
			"updatedAt":   item.UpdatedAt,
		})
	}

	var recentDatabases []model.AssetDatabase
	if err := s.db.Order("updated_at DESC").Limit(6).Find(&recentDatabases).Error; err != nil {
		return nil, err
	}
	databaseItems := make([]map[string]any, 0, len(recentDatabases))
	for _, item := range recentDatabases {
		databaseItems = append(databaseItems, map[string]any{
			"id":            item.ID,
			"name":          item.Name,
			"dbType":        item.DBType,
			"host":          item.Host,
			"port":          item.Port,
			"dbName":        item.DBName,
			"version":       item.Version,
			"status":        item.Status,
			"connectStatus": item.ConnectStatus,
			"updatedAt":     item.UpdatedAt,
		})
	}

	// Recent clusters likewise ride the V2 connection source (I-a S6).
	recentClusters := make([]infraModel.ProviderConnection, len(k8sConns))
	copy(recentClusters, k8sConns)
	sort.Slice(recentClusters, func(i, j int) bool {
		return recentClusters[i].UpdatedAt.After(recentClusters[j].UpdatedAt)
	})
	if len(recentClusters) > 6 {
		recentClusters = recentClusters[:6]
	}
	clusterItems := make([]map[string]any, 0, len(recentClusters))
	for i := range recentClusters {
		item := &recentClusters[i]
		nodeCount := 0
		if parsed, ok := k8sConfigUintValue(item, "node_count"); ok {
			nodeCount = int(parsed)
		}
		clusterItems = append(clusterItems, map[string]any{
			"id":        item.SourceID,
			"name":      item.Name,
			"status":    item.Status,
			"apiServer": item.Endpoint,
			"version":   item.Version,
			"nodeCount": nodeCount,
			"updatedAt": item.UpdatedAt,
		})
	}

	return map[string]any{
		"summary": map[string]any{
			"hostTotal":           hostTotal,
			"hostOnline":          hostOnline,
			"hostOffline":         hostOffline,
			"groupTotal":          groupTotal,
			"credentialTotal":     credentialTotal,
			"credentialEnabled":   credentialEnabled,
			"cloudAccountTotal":   cloudAccountTotal,
			"cloudAccountEnabled": cloudAccountEnabled,
			"databaseTotal":       databaseTotal,
			"databaseHealthy":     databaseHealthy,
			"k8sClusterTotal":     k8sClusterTotal,
			"k8sClusterOnline":    k8sClusterOnline,
			"k8sNodeTotal":        k8sNodeTotal,
		},
		"health": map[string]any{
			"offlineHosts":        hostOffline,
			"authFailedHosts":     hostAuthFailed,
			"healthyDatabases":    databaseHealthy,
			"abnormalDatabases":   databaseTotal - databaseHealthy,
			"abnormalClusters":    k8sClusterTotal - k8sClusterOnline,
			"incompleteAssets":    incompleteHosts + incompleteDatabases + incompleteClusters,
			"incompleteHosts":     incompleteHosts,
			"incompleteDatabases": incompleteDatabases,
			"incompleteClusters":  incompleteClusters,
		},
		"distributions": map[string]any{
			"providers":    providerRows,
			"environments": environmentRows,
		},
		"topGroups":       groupItems,
		"recentHosts":     hostItems,
		"recentDatabases": databaseItems,
		"recentClusters":  clusterItems,
	}, nil
}

func (s *Service) paginateLogs(entity any, pageNum, pageSize int, field string, keyword string) (map[string]any, error) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	query := s.db.Model(entity)
	if keyword != "" {
		query = query.Where(field+" like ?", "%"+keyword+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	switch entity.(type) {
	case *model.LoginLog:
		var list []model.LoginLog
		if err := query.Order("id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
			return nil, err
		}
		return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
	case *model.OperationLog:
		var list []model.OperationLog
		if err := query.Order("id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
			return nil, err
		}
		return map[string]any{"list": list, "total": total, "pageNum": pageNum, "pageSize": pageSize}, nil
	default:
		return nil, errors.New("unsupported entity")
	}
}
