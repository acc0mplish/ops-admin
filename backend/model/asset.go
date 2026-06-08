package model

import "time"

type AssetHostGroup struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	ParentID    uint      `json:"parentId" gorm:"index;default:0"`
	Name        string    `json:"name" gorm:"size:128;not null"`
	Code        string    `json:"code" gorm:"size:128"`
	Sort        int       `json:"sort" gorm:"default:0"`
	Status      int       `json:"status" gorm:"default:1;not null"`
	Description string    `json:"description" gorm:"size:255"`
	CreatedAt   time.Time `json:"createTime"`
	UpdatedAt   time.Time `json:"updateTime"`
}

func (AssetHostGroup) TableName() string {
	return "asset_host_group"
}

type AssetHostGroupRelation struct {
	HostID  uint `json:"hostId" gorm:"primaryKey"`
	GroupID uint `json:"groupId" gorm:"primaryKey"`
}

func (AssetHostGroupRelation) TableName() string {
	return "asset_host_group_rel"
}

type AssetCredential struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"size:128;not null"`
	AuthType    string    `json:"authType" gorm:"size:32;not null;index"`
	Username    string    `json:"username" gorm:"size:128;not null"`
	Password    string    `json:"password,omitempty" gorm:"size:512"`
	PrivateKey  string    `json:"privateKey,omitempty" gorm:"type:text"`
	Passphrase  string    `json:"passphrase,omitempty" gorm:"size:512"`
	Status      int       `json:"status" gorm:"default:1;not null"`
	Description string    `json:"description" gorm:"size:255"`
	CreatedAt   time.Time `json:"createTime"`
	UpdatedAt   time.Time `json:"updateTime"`
}

func (AssetCredential) TableName() string {
	return "asset_credential"
}

type AssetCloudAccount struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"size:128;not null"`
	Provider    string    `json:"provider" gorm:"size:64;not null;index"`
	AccessKey   string    `json:"accessKey" gorm:"size:255"`
	SecretKey   string    `json:"secretKey,omitempty" gorm:"size:512"`
	Region      string    `json:"region" gorm:"size:128"`
	Status      int       `json:"status" gorm:"default:1;not null"`
	Description string    `json:"description" gorm:"size:255"`
	CreatedAt   time.Time `json:"createTime"`
	UpdatedAt   time.Time `json:"updateTime"`
}

func (AssetCloudAccount) TableName() string {
	return "asset_cloud_account"
}

type AssetHost struct {
	ID             uint              `json:"id" gorm:"primaryKey"`
	HostName       string            `json:"hostName" gorm:"size:128;not null;index"`
	Alias          string            `json:"alias" gorm:"size:128"`
	GroupID        uint              `json:"groupId" gorm:"index"`
	Group          AssetHostGroup    `json:"group" gorm:"foreignKey:GroupID"`
	HostGroups     []AssetHostGroup  `json:"hostGroups" gorm:"many2many:asset_host_group_rel;joinForeignKey:HostID;joinReferences:GroupID"`
	CredentialID   uint              `json:"credentialId" gorm:"index"`
	Credential     AssetCredential   `json:"credential" gorm:"foreignKey:CredentialID"`
	CloudAccountID *uint             `json:"cloudAccountId" gorm:"index"`
	CloudAccount   AssetCloudAccount `json:"cloudAccount" gorm:"foreignKey:CloudAccountID"`
	PrivateIP      string            `json:"privateIp" gorm:"size:64;index"`
	PublicIP       string            `json:"publicIp" gorm:"size:64;index"`
	SSHUser        string            `json:"sshUser" gorm:"size:128"`
	SSHIP          string            `json:"sshIp" gorm:"size:64;index"`
	SSHPort        int               `json:"sshPort" gorm:"default:22"`
	OS             string            `json:"os" gorm:"size:128"`
	Arch           string            `json:"arch" gorm:"size:64"`
	CPU            string            `json:"cpu" gorm:"size:64"`
	Memory         string            `json:"memory" gorm:"size:64"`
	Disk           string            `json:"disk" gorm:"size:64"`
	Environment    string            `json:"environment" gorm:"size:64;index"`
	Provider       string            `json:"provider" gorm:"size:64"`
	Region         string            `json:"region" gorm:"size:128"`
	InstanceID     string            `json:"instanceId" gorm:"size:128;index"`
	Status         int               `json:"status" gorm:"default:1;not null"`
	AliveStatus    int               `json:"aliveStatus" gorm:"default:2;not null"`
	AuthStatus     int               `json:"authStatus" gorm:"default:2;not null"`
	Description    string            `json:"description" gorm:"size:255"`
	LastCheckTime  *time.Time        `json:"lastCheckTime"`
	CreatedAt      time.Time         `json:"createTime"`
	UpdatedAt      time.Time         `json:"updateTime"`
}

func (AssetHost) TableName() string {
	return "asset_host"
}

type AssetDatabase struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	Name          string     `json:"name" gorm:"size:128;not null;index"`
	DBType        string     `json:"dbType" gorm:"size:32;not null;index"`
	Host          string     `json:"host" gorm:"size:128;not null;index"`
	Port          int        `json:"port" gorm:"default:3306"`
	Username      string     `json:"username" gorm:"size:128;not null"`
	Password      string     `json:"password,omitempty" gorm:"size:512"`
	DBName        string     `json:"dbName" gorm:"size:128"`
	Charset       string     `json:"charset" gorm:"size:64"`
	Version       string     `json:"version" gorm:"size:128"`
	Status        int        `json:"status" gorm:"default:1;not null"`
	ConnectStatus int        `json:"connectStatus" gorm:"default:0;not null"`
	Description   string     `json:"description" gorm:"size:255"`
	LastCheckTime *time.Time `json:"lastCheckTime"`
	CreatedAt     time.Time  `json:"createTime"`
	UpdatedAt     time.Time  `json:"updateTime"`
}

func (AssetDatabase) TableName() string {
	return "asset_database"
}
