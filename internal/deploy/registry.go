package deploy

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
)

const PinnedSingBoxVersion = "1.13.13"

var pinnedSingBoxChecksums = map[string]string{
	"linux-amd64": "bb99cabf47694625db421ee17898f36cdc1f9c2cb5decf65b12bac8d8437e842",
	"linux-arm64": "d7fab87b921933eb281d8ee7bd5377cdd8228089f1f7c807c9363a6a2329286c",
}

//go:embed templates/*.sh
var templateFS embed.FS

type Registry struct {
	templates map[string]Template
}

type Template struct {
	Name           string              `json:"name"`
	Version        string              `json:"version"`
	Description    string              `json:"description"`
	Checksum       string              `json:"checksum"`
	Content        []byte              `json:"-"`
	UserSelectable bool                `json:"user_selectable"`
	ExecutionMode  string              `json:"execution_mode"`
	Parameters     []TemplateParameter `json:"parameters,omitempty"`
	Runtime        RuntimeMetadata     `json:"runtime"`
}

type RuntimeMetadata struct {
	Name      string            `json:"name"`
	Version   string            `json:"version"`
	Channel   string            `json:"channel"`
	Checksums map[string]string `json:"checksums,omitempty"`
}

type TemplateParameter struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Default     string   `json:"default,omitempty"`
	Options     []string `json:"options,omitempty"`
	Secret      bool     `json:"secret,omitempty"`
	Advanced    bool     `json:"advanced,omitempty"`
	Description string   `json:"description,omitempty"`
}

func BuiltinTemplates() Registry {
	probe := mustTemplate("probe-system", "1.0.0", "templates/probe-system.sh", false, "stdin", RuntimeMetadata{})
	probe.Description = "Detects OS, architecture, privilege, systemd, firewall, and selected proxy-port state."
	probe.Parameters = []TemplateParameter{
		{
			Name:        "proxy_port",
			Label:       "Proxy port",
			Type:        "integer",
			Default:     "443",
			Description: "Remote TCP port checked for existing listeners.",
		},
	}

	vless := mustTemplate("singbox-vless-reality", "1.0.0", "templates/singbox-vless-reality.sh", true, "uploaded_script", RuntimeMetadata{
		Name:      "sing-box",
		Version:   PinnedSingBoxVersion,
		Channel:   "official-stable",
		Checksums: pinnedSingBoxChecksums,
	})
	vless.Description = "Installs a pinned official sing-box VLESS Reality TCP node."
	vless.Parameters = []TemplateParameter{
		{
			Name:        "node_name",
			Label:       "Node name",
			Type:        "string",
			Default:     "deployed-vless-reality",
			Description: "Generated node tag used when importing the deployment result.",
		},
		{
			Name:        "node_server",
			Label:       "Node server",
			Type:        "string",
			Required:    true,
			Description: "Public server address written into the generated node.",
		},
		{
			Name:        "proxy_port",
			Label:       "Proxy port",
			Type:        "integer",
			Default:     "443",
			Description: "Remote VLESS Reality listen port.",
		},
		{
			Name:        "runtime_source",
			Label:       "Runtime source",
			Type:        "select",
			Default:     "remote",
			Options:     []string{"remote", "cache"},
			Description: "Use remote download or upload a checksum-verified local runtime cache archive.",
		},
		{
			Name:        "debug_preserve_remote_run_dir",
			Label:       "Preserve remote run directory",
			Type:        "boolean",
			Default:     "false",
			Advanced:    true,
			Description: "Keep the remote temporary directory for troubleshooting.",
		},
		{
			Name:        "sbm_uuid",
			Label:       "VLESS UUID",
			Type:        "string",
			Secret:      true,
			Advanced:    true,
			Description: "Optional VLESS user UUID; generated when omitted.",
		},
		{
			Name:        "reality_short_id",
			Label:       "Reality short ID",
			Type:        "string",
			Secret:      true,
			Advanced:    true,
			Description: "Optional Reality short ID; generated when omitted.",
		},
		{
			Name:        "reality_server_name",
			Label:       "Reality server name",
			Type:        "string",
			Default:     "www.microsoft.com",
			Advanced:    true,
			Description: "Reality handshake server_name.",
		},
		{
			Name:        "reality_private_key",
			Label:       "Reality private key",
			Type:        "string",
			Secret:      true,
			Advanced:    true,
			Description: "Optional Reality private key; must be supplied with the matching public key.",
		},
		{
			Name:        "reality_public_key",
			Label:       "Reality public key",
			Type:        "string",
			Advanced:    true,
			Description: "Optional Reality public key; must be supplied with the matching private key.",
		},
	}

	security := mustTemplate("security-basic", "1.0.0", "templates/security-basic.sh", true, "uploaded_script", RuntimeMetadata{})
	security.Description = "Runs a low-risk security baseline inspection and optional allowlisted base-package install without changing SSH policy."
	security.Parameters = []TemplateParameter{
		{
			Name:        "security_install_base_packages",
			Label:       "Install base packages",
			Type:        "boolean",
			Default:     "false",
			Description: "When true, installs only the allowlisted base packages using root or passwordless sudo.",
		},
		{
			Name:        "security_base_packages",
			Label:       "Base packages",
			Type:        "string",
			Default:     "curl ca-certificates tar gzip unzip",
			Advanced:    true,
			Description: "Space-separated package allowlist for the optional base-package install step.",
		},
		{
			Name:        "security_firewall_mode",
			Label:       "Firewall mode",
			Type:        "select",
			Default:     "inspect_only",
			Options:     []string{"inspect_only"},
			Description: "Firewall handling is read-only in the first version.",
		},
	}

	return Registry{templates: map[string]Template{
		probe.Name:    probe,
		vless.Name:    vless,
		security.Name: security,
	}}
}

func RuntimeChecksum(runtimeName, version, osName, arch string) (string, bool) {
	if runtimeName != "sing-box" || version != PinnedSingBoxVersion {
		return "", false
	}
	checksum, ok := pinnedSingBoxChecksums[fmt.Sprintf("%s-%s", osName, arch)]
	return checksum, ok
}

func ValidatePinnedRuntime(runtimeName, version string) error {
	if runtimeName != "sing-box" || version != PinnedSingBoxVersion {
		return fmt.Errorf("runtime cache only supports sing-box %s", PinnedSingBoxVersion)
	}
	return nil
}

func (r Registry) Get(name string) (Template, bool) {
	template, ok := r.templates[name]
	return template, ok
}

func (r Registry) List() []Template {
	templates := make([]Template, 0, len(r.templates))
	for _, template := range r.templates {
		templates = append(templates, template)
	}
	return templates
}

func mustTemplate(name, version, path string, userSelectable bool, executionMode string, runtime RuntimeMetadata) Template {
	content, err := templateFS.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("read embedded template %s: %v", path, err))
	}
	return Template{
		Name:           name,
		Version:        version,
		Checksum:       checksum(content),
		Content:        content,
		UserSelectable: userSelectable,
		ExecutionMode:  executionMode,
		Runtime:        runtime,
	}
}

func checksum(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
