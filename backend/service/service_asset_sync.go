package service

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"ops-admin/backend/model"
)

func (s *Service) SyncAssetHost(id uint) (*model.AssetHost, error) {
	var host model.AssetHost
	if err := s.db.Preload("Credential").Preload("Gateway").Preload("Gateway.Credential").First(&host, id).Error; err != nil {
		return nil, err
	}

	deadline := time.Now().Add(10 * time.Second)
	now := time.Now()
	updates := map[string]any{
		"alive_status":    2,
		"auth_status":     2,
		"last_check_time": &now,
	}

	address := net.JoinHostPort(host.SSHIP, strconv.Itoa(host.SSHPort))
	connectTimeout := remainingTimeout(deadline, 3*time.Second)
	if normalizeConnectionMode(host.ConnectionMode) == "gateway" && host.GatewayID != nil && *host.GatewayID > 0 {
		if connectTimeout > 0 {
			ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
			if conn, cleanup, err := s.dialThroughGateway(ctx, *host.GatewayID, "tcp", address); err == nil {
				_ = conn.Close()
				cleanup()
				updates["alive_status"] = 1
			}
			cancel()
		}
	} else if connectTimeout > 0 {
		conn, err := net.DialTimeout("tcp", address, connectTimeout)
		if err == nil {
			_ = conn.Close()
			updates["alive_status"] = 1
		}
	}

	sshTimeout := remainingTimeout(deadline, 5*time.Second)
	if sshTimeout <= 0 {
		_ = s.db.Model(&host).Updates(updates).Error
		return s.GetAssetHost(id)
	}
	client, err := s.newSSHClientWithTimeout(host, sshTimeout)
	if err != nil {
		if time.Until(deadline) <= 0 {
			updates["alive_status"] = 2
		}
		_ = s.db.Model(&host).Updates(updates).Error
		return s.GetAssetHost(id)
	}
	defer client.Close()
	updates["auth_status"] = 1
	updates["status"] = 1

	info, timedOut := collectHostInfo(client, deadline)
	if timedOut || time.Until(deadline) <= 0 {
		updates["alive_status"] = 2
	}
	if len(info) > 0 {
		for key, value := range info {
			if strings.TrimSpace(value) != "" {
				updates[key] = value
			}
		}
	}

	if err := s.db.Model(&host).Updates(updates).Error; err != nil {
		return nil, err
	}
	s.recordAssetChange("host", host.ID, host.HostName, "sync", "Synchronize Host Configuration and Connection Status", "system")
	return s.GetAssetHost(id)
}

func (s *Service) ImportAssetHosts(groupID uint, rows []AssetHostImportRow) (map[string]any, error) {
	var group model.AssetHostGroup
	if err := s.db.First(&group, groupID).Error; err != nil {
		return nil, errors.New("host group not found")
	}

	successCount := 0
	failed := make([]string, 0)

	for _, row := range rows {
		hostName := Trimmed(row.HostName)
		sshIP := Trimmed(row.SSHIP)
		sshUser := Trimmed(row.SSHUser)
		credentialName := Trimmed(row.CredentialName)
		environment := normalizeEnvCode(row.Environment)
		connectionMode := normalizeConnectionMode(row.ConnectionMode)
		if hostName == "" || sshIP == "" || sshUser == "" || credentialName == "" || environment == "" {
			failed = append(failed, hostName+"(missing fields)")
			continue
		}

		var credential model.AssetCredential
		if err := s.db.Where("name = ? AND status = ?", credentialName, 1).First(&credential).Error; err != nil {
			failed = append(failed, hostName+"(credential not found)")
			continue
		}
		var gatewayID *uint
		if connectionMode == "gateway" {
			gatewayName := Trimmed(row.GatewayName)
			if gatewayName == "" {
				failed = append(failed, hostName+"(gateway is required)")
				continue
			}
			var gateway model.AssetGateway
			if err := s.db.Where("name = ? AND status = ?", gatewayName, 1).First(&gateway).Error; err != nil {
				failed = append(failed, hostName+"(gateway not found)")
				continue
			}
			gatewayID = &gateway.ID
		}

		var existing model.AssetHost
		if err := s.db.Where("ssh_ip = ?", sshIP).First(&existing).Error; err == nil {
			failed = append(failed, hostName+"(ssh ip exists)")
			continue
		}

		host := model.AssetHost{
			HostName:       hostName,
			GroupID:        groupID,
			CredentialID:   optionalUint(credential.ID),
			ConnectionMode: connectionMode,
			GatewayID:      gatewayID,
			SSHIP:          sshIP,
			SSHUser:        sshUser,
			SSHPort:        defaultSSHPort(row.SSHPort),
			PrivateIP:      Trimmed(row.PrivateIP),
			PublicIP:       Trimmed(row.PublicIP),
			Environment:    environment,
			Status:         1,
			AuthStatus:     2,
			AliveStatus:    2,
			Description:    Trimmed(row.Description),
			Provider:       firstNonEmpty(Trimmed(row.Provider), "On-premises"),
			Region:         Trimmed(row.Region),
		}
		if err := s.db.Create(&host).Error; err != nil {
			failed = append(failed, hostName+"(create failed)")
			continue
		}
		_ = syncAssetHostGroups(s.db, host.ID, []uint{groupID}, groupID)
		successCount++
	}

	return map[string]any{
		"success":     successCount,
		"fail":        len(failed),
		"total":       len(rows),
		"failedHosts": failed,
	}, nil
}

func (s *Service) SyncAssetHostsFromCloud(payload AssetCloudSyncPayload) (map[string]any, error) {
	provider := strings.ToLower(Trimmed(payload.Provider))
	if payload.GroupID == 0 {
		return nil, errors.New("groupId is required")
	}
	credentialID := optionalUint(payload.CredentialID)
	if credentialID == nil {
		return nil, errors.New("select an authentication credential")
	}
	var credential model.AssetCredential
	if err := s.db.Where("id = ? AND status = ?", *credentialID, 1).First(&credential).Error; err != nil {
		return nil, errors.New("selected authentication credential does not exist or is disabled")
	}
	environment := normalizeEnvCode(payload.Environment)
	if environment == "" {
		return nil, errors.New("select an environment")
	}
	connectionMode := normalizeConnectionMode(payload.ConnectionMode)
	gatewayID := optionalGatewayID(connectionMode, payload.GatewayID)
	if err := validateGatewaySelection(connectionMode, gatewayID); err != nil {
		return nil, err
	}
	if gatewayID != nil {
		var gateway model.AssetGateway
		if err := s.db.Where("id = ? AND status = ?", *gatewayID, 1).First(&gateway).Error; err != nil {
			return nil, errors.New("selected access gateway does not exist or is disabled")
		}
	}
	if !payload.UseExistingAccount {
		return nil, errors.New("cloud sync requires a configured cloud account with at least one region")
	}

	accessKey := Trimmed(payload.AccessKey)
	secretKey := Trimmed(payload.SecretKey)
	regions := normalizeCloudRegions(payload.Regions, payload.Region)
	region := strings.Join(regions, ",")
	var accountID *uint

	if payload.UseExistingAccount {
		var account model.AssetCloudAccount
		if err := s.db.First(&account, payload.CloudAccountID).Error; err != nil {
			return nil, errors.New("cloud account not found")
		}
		provider = strings.ToLower(Trimmed(account.Provider))
		accessKey = Trimmed(account.AccessKey)
		secretKey = Trimmed(account.SecretKey)
		if len(regions) == 0 {
			regions = normalizeCloudRegions(account.Regions, account.Region)
			region = strings.Join(regions, ",")
		}
		accountID = &account.ID
	}

	instances, err := fetchCloudInstances(provider, accessKey, secretKey, region)
	if err != nil {
		return nil, err
	}
	if len(instances) == 0 {
		return nil, errors.New("no instances found")
	}

	added := 0
	updated := 0
	skipped := 0
	addedHosts := make([]string, 0)
	updatedHosts := make([]string, 0)
	skippedHosts := make([]string, 0)
	regionCounts := make(map[string]int)
	for _, item := range instances {
		regionCounts[firstNonEmpty(item.Region, "unknown")]++
		// Cloud hosts should be managed on their VPC address first. A public address
		// remains inventory metadata and is only a fallback when no private address exists.
		sshIP := firstNonEmpty(item.PrivateIP, item.PublicIP)
		if sshIP == "" {
			skipped++
			skippedHosts = append(skippedHosts, firstNonEmpty(item.HostName, item.InstanceID, "unknown")+": no public or private IP was returned")
			continue
		}

		var host model.AssetHost
		lookup := s.db.Model(&model.AssetHost{})
		if accountID != nil && item.InstanceID != "" {
			// A manually managed host may use the same IP. Only a record from the
			// same cloud account with the same cloud instance ID is an idempotent match.
			lookup = lookup.Where("cloud_account_id = ? AND instance_id = ?", *accountID, item.InstanceID)
		} else if item.InstanceID != "" {
			lookup = lookup.Where("provider = ? AND instance_id = ?", provider, item.InstanceID)
		} else if accountID != nil {
			lookup = lookup.Where("cloud_account_id = ? AND ssh_ip = ?", *accountID, sshIP)
		} else {
			lookup = lookup.Where("provider = ? AND ssh_ip = ?", provider, sshIP)
		}
		err := lookup.First(&host).Error
		switch {
		case err == nil:
			// Discovery must never overwrite an existing host. Cloud credentials,
			// routing, environment and manually maintained host metadata stay intact.
			skipped++
			skippedHosts = append(skippedHosts, firstNonEmpty(item.HostName, item.InstanceID, sshIP)+": host already exists and was not overwritten")
		case errors.Is(err, gorm.ErrRecordNotFound):
			newHost := model.AssetHost{
				HostName:       firstNonEmpty(item.HostName, item.InstanceID, sshIP),
				GroupID:        payload.GroupID,
				PrivateIP:      item.PrivateIP,
				PublicIP:       item.PublicIP,
				SSHIP:          sshIP,
				SSHUser:        firstNonEmpty(item.SSHUser, "root"),
				SSHPort:        defaultSSHPort(item.SSHPort),
				OS:             item.OS,
				CPU:            item.CPU,
				Memory:         item.Memory,
				Disk:           item.Disk,
				Provider:       provider,
				Region:         item.Region,
				InstanceID:     item.InstanceID,
				CloudAccountID: accountID,
				CredentialID:   credentialID,
				ConnectionMode: connectionMode,
				GatewayID:      gatewayID,
				Environment:    environment,
				Status:         1,
				AliveStatus:    2,
				AuthStatus:     2,
			}
			if createErr := s.db.Create(&newHost).Error; createErr == nil {
				_ = syncAssetHostGroups(s.db, newHost.ID, []uint{payload.GroupID}, payload.GroupID)
				added++
				addedHosts = append(addedHosts, newHost.HostName)
			} else {
				skipped++
				skippedHosts = append(skippedHosts, newHost.HostName+": creation failed: "+createErr.Error())
			}
		default:
			skipped++
			skippedHosts = append(skippedHosts, firstNonEmpty(item.HostName, item.InstanceID, sshIP)+": failed to query existing host")
		}
	}

	return map[string]any{
		"provider":     provider,
		"total":        len(instances),
		"added":        added,
		"addedHosts":   addedHosts,
		"updated":      updated,
		"updatedHosts": updatedHosts,
		"skipped":      skipped,
		"skippedHosts": skippedHosts,
		"regionCounts": regionCounts,
	}, nil
}
