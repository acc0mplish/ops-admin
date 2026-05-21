package util

import (
	"fmt"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	cvm "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cvm/v20170312"
)

type TencentCloudService struct {
	AccessKey    string
	AccessSecret string
}

type TencentInstanceInfo struct {
	InstanceID string
	HostName   string
	PrivateIP  string
	PublicIP   string
	CPU        int
	Memory     int
	Disk       int
	OS         string
	Region     string
}

func NewTencentCloudService(accessKey, accessSecret string) *TencentCloudService {
	return &TencentCloudService{
		AccessKey:    accessKey,
		AccessSecret: accessSecret,
	}
}

func (s *TencentCloudService) GetInstances() ([]TencentInstanceInfo, error) {
	credential := common.NewCredential(s.AccessKey, s.AccessSecret)
	clientProfile := profile.NewClientProfile()
	clientProfile.HttpProfile.Endpoint = "cvm.tencentcloudapi.com"

	regionClient, err := cvm.NewClient(credential, "ap-guangzhou", clientProfile)
	if err != nil {
		return nil, fmt.Errorf("create tencent region client failed: %w", err)
	}
	regionResponse, err := regionClient.DescribeRegions(cvm.NewDescribeRegionsRequest())
	if err != nil {
		return nil, fmt.Errorf("describe tencent regions failed: %w", err)
	}

	var allInstances []TencentInstanceInfo
	for _, regionInfo := range regionResponse.Response.RegionSet {
		if regionInfo.Region == nil {
			continue
		}
		regionName := *regionInfo.Region
		client, err := cvm.NewClient(credential, regionName, clientProfile)
		if err != nil {
			continue
		}
		response, err := client.DescribeInstances(cvm.NewDescribeInstancesRequest())
		if err != nil {
			continue
		}
		for _, instance := range response.Response.InstanceSet {
			item := TencentInstanceInfo{Region: regionName}
			if instance.InstanceId != nil {
				item.InstanceID = *instance.InstanceId
			}
			if instance.InstanceName != nil {
				item.HostName = *instance.InstanceName
			}
			if instance.OsName != nil {
				item.OS = *instance.OsName
			}
			if instance.CPU != nil {
				item.CPU = int(*instance.CPU)
			}
			if instance.Memory != nil {
				item.Memory = int(*instance.Memory)
			}
			if instance.SystemDisk != nil && instance.SystemDisk.DiskSize != nil {
				item.Disk += int(*instance.SystemDisk.DiskSize)
			}
			if instance.DataDisks != nil {
				for _, disk := range instance.DataDisks {
					if disk.DiskSize != nil {
						item.Disk += int(*disk.DiskSize)
					}
				}
			}
			if instance.PrivateIpAddresses != nil && len(instance.PrivateIpAddresses) > 0 && instance.PrivateIpAddresses[0] != nil {
				item.PrivateIP = *instance.PrivateIpAddresses[0]
			}
			if instance.PublicIpAddresses != nil && len(instance.PublicIpAddresses) > 0 && instance.PublicIpAddresses[0] != nil {
				item.PublicIP = *instance.PublicIpAddresses[0]
			}
			allInstances = append(allInstances, item)
		}
	}
	return allInstances, nil
}
