package service

import "testing"

func TestAliyunCloudInstancesMapsECSFields(t *testing.T) {
	item := aliyunECSInstance{InstanceID: "i-example", InstanceName: "game-node", CPU: 4, Memory: 8192, OSName: "Ubuntu", RegionID: "cn-hangzhou"}
	item.PublicIPAddress.IPAddress = []string{"47.1.2.3"}
	item.VPCAttributes.PrivateIPAddress.IPAddress = []string{"192.168.1.10"}
	item.SystemDisk.Size = 40
	item.DataDisks.Disk = []struct {
		Size int "json:\"Size\""
	}{{Size: 100}}
	instances := aliyunCloudInstances([]aliyunECSInstance{item}, "")
	if len(instances) != 1 {
		t.Fatalf("expected one instance, got %d", len(instances))
	}
	result := instances[0]
	if result.InstanceID != "i-example" || result.HostName != "game-node" || result.PrivateIP != "192.168.1.10" || result.PublicIP != "47.1.2.3" {
		t.Fatalf("unexpected instance mapping: %#v", result)
	}
	if result.CPU != "4 vCPU" || result.Memory != "8 GB" || result.Disk != "140 GB" || result.Region != "cn-hangzhou" {
		t.Fatalf("unexpected capacity mapping: %#v", result)
	}
}

func TestAliyunECSEndpointUsesConfiguredRegion(t *testing.T) {
	if endpoint := aliyunECSEndpoint("cn-guangzhou"); endpoint != "https://ecs.cn-guangzhou.aliyuncs.com/" {
		t.Fatalf("unexpected regional endpoint: %s", endpoint)
	}
	if endpoint := aliyunECSEndpoint(""); endpoint != aliyunECSAPIEndpoint {
		t.Fatalf("unexpected default endpoint: %s", endpoint)
	}

}

func TestNormalizeCloudRegionsSupportsMultipleConfiguredRegions(t *testing.T) {
	regions := normalizeCloudRegions([]string{"cn-guangzhou", "ap-southeast-1"}, "cn-guangzhou,cn-shanghai")
	want := []string{"cn-guangzhou", "ap-southeast-1", "cn-shanghai"}
	if len(regions) != len(want) {
		t.Fatalf("unexpected regions: %#v", regions)
	}
	for i := range want {
		if regions[i] != want[i] {
			t.Fatalf("unexpected regions: %#v", regions)
		}
	}
}

func TestAliyunCloudInstancesUsesClassicNetworkPrivateIP(t *testing.T) {
	item := aliyunECSInstance{InstanceID: "i-inner", InstanceName: "private-only"}
	item.InnerIPAddress.IPAddress = []string{"10.0.2.15"}
	instances := aliyunCloudInstances([]aliyunECSInstance{item}, "cn-guangzhou")
	if len(instances) != 1 || instances[0].PrivateIP != "10.0.2.15" {
		t.Fatalf("expected classic-network private IP, got %#v", instances)
	}
}
