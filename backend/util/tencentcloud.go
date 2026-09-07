package util

import (
	"fmt"
	"net/url"
	"strings"

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

func (s *TencentCloudService) GetInstances(regions []string) ([]TencentInstanceInfo, error) {
	if len(regions) == 0 {
		return nil, fmt.Errorf("at least one Tencent Cloud region is required")
	}
	credential := common.NewCredential(s.AccessKey, s.AccessSecret)
	clientProfile := profile.NewClientProfile()
	clientProfile.HttpProfile.Endpoint = "cvm.tencentcloudapi.com"
	// E-2 (M11) — development-only mock endpoint override for the §15 pair
	// capture, the same gate and precedence the aliyun legacy hook and the V2
	// adapters apply (override above the provider default; inert outside
	// GO_ENV=development + a loopback http URL — 판정 J7).
	if override, ok := CloudEndpointOverride("tencent"); ok {
		if parsed, err := url.Parse(override); err == nil {
			clientProfile.HttpProfile.Endpoint = parsed.Host
			clientProfile.HttpProfile.Scheme = "HTTP"
		}
	}

	var allInstances []TencentInstanceInfo
	var failures []string
	for _, configuredRegion := range regions {
		regionName := strings.TrimSpace(configuredRegion)
		if regionName == "" {
			continue
		}
		client, err := cvm.NewClient(credential, regionName, clientProfile)
		if err != nil {
			failures = append(failures, regionName+": "+err.Error())
			continue
		}
		response, err := client.DescribeInstances(cvm.NewDescribeInstancesRequest())
		if err != nil {
			failures = append(failures, regionName+": "+err.Error())
			continue
		}
		// ④review LOW-1 — a 2xx response that the SDK did not classify (an
		// Error envelope with an empty code, a gateway body without the
		// "Response" wrapper) decodes to a nil typed response: classify it as
		// a region failure instead of dereferencing nil or reporting a silent
		// empty success.
		if response == nil || response.Response == nil {
			failures = append(failures, regionName+": Tencent Cloud returned a 2xx response without a usable response envelope (classified as a query failure)")
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
	if len(allInstances) == 0 && len(failures) > 0 {
		return nil, fmt.Errorf("Tencent Cloud instance query failed: %s", strings.Join(failures, "; "))
	}
	return allInstances, nil
}
