package models

import "time"

const (
	DeploymentRunStatusPending   = "pending"
	DeploymentRunStatusRunning   = "running"
	DeploymentRunStatusSuccess   = "success"
	DeploymentRunStatusFailed    = "failed"
	DeploymentRunStatusCancelled = "cancelled"
)

const (
	DeploymentConnectionDirect       = "direct"
	DeploymentConnectionManagedProxy = "managed_proxy"
	DeploymentConnectionCustomProxy  = "custom_proxy"
)

// DeploymentRun records a proxy-node deployment attempt without storing credentials.
type DeploymentRun struct {
	ID                  string     `gorm:"primaryKey;type:text" json:"id"`
	TaskID              string     `gorm:"type:text;index" json:"task_id,omitempty"`
	TemplateName        string     `gorm:"type:text;index" json:"template_name"`
	TemplateVersion     string     `gorm:"type:text" json:"template_version"`
	TemplateChecksum    string     `gorm:"type:text" json:"template_checksum"`
	Status              string     `gorm:"type:text;index" json:"status"`
	DryRun              bool       `gorm:"default:false" json:"dry_run"`
	SSHHost             string     `gorm:"type:text" json:"ssh_host"`
	SSHPort             int        `json:"ssh_port"`
	SSHUser             string     `gorm:"type:text" json:"ssh_user"`
	AuthMethod          string     `gorm:"type:text" json:"auth_method"`
	NodeServer          string     `gorm:"type:text" json:"node_server"`
	NodeProxyPort       int        `json:"proxy_port"`
	ConnectionMode      string     `gorm:"type:text" json:"connection_mode"`
	ProxyType           string     `gorm:"type:text" json:"proxy_type,omitempty"`
	ProxyHost           string     `gorm:"type:text" json:"proxy_host,omitempty"`
	ConnectionProxyPort int        `json:"proxy_connection_port,omitempty"`
	RedactedParams      JSONMap    `gorm:"type:text" json:"redacted_params,omitempty"`
	ProgressMarkers     JSONMap    `gorm:"type:text" json:"progress_markers,omitempty"`
	ProbeResult         JSONMap    `gorm:"type:text" json:"probe_result,omitempty"`
	GeneratedNode       JSONMap    `gorm:"type:text" json:"generated_node,omitempty"`
	Stdout              string     `gorm:"type:text" json:"stdout,omitempty"`
	Stderr              string     `gorm:"type:text" json:"stderr,omitempty"`
	ExitCode            *int       `json:"exit_code,omitempty"`
	ImportedNodeID      *uint      `json:"imported_node_id,omitempty"`
	StartedAt           *time.Time `json:"started_at,omitempty"`
	CompletedAt         *time.Time `json:"completed_at,omitempty"`
	CreatedAt           time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt           time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (DeploymentRun) TableName() string {
	return "deployment_runs"
}
