package daemon

import (
	"bytes"
	"strings"
	"testing"
	"text/template"
)

func renderSystemd(t *testing.T, tmplStr string, cfg SystemdConfig) string {
	t.Helper()
	tmpl, err := template.New("x").Parse(tmplStr)
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		t.Fatalf("execute template: %v", err)
	}
	return buf.String()
}

func TestSystemdTemplate_KeepAliveControlsRestart(t *testing.T) {
	base := SystemdConfig{
		SbmPath: "/usr/bin/sbm", DataDir: "/var/sbm", Port: "19090",
		LogPath: "/var/log/sbm", WorkingDir: "/var/sbm", HomeDir: "/root",
		RunAtLoad: true,
	}

	cases := []struct {
		name     string
		tmpl     string
		keep     bool
		restart  string
	}{
		{"system/keepalive", systemdTemplate, true, "Restart=always"},
		{"system/no-keepalive", systemdTemplate, false, "Restart=no"},
		{"user/keepalive", systemdUserTemplate, true, "Restart=always"},
		{"user/no-keepalive", systemdUserTemplate, false, "Restart=no"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base
			cfg.KeepAlive = tc.keep
			out := renderSystemd(t, tc.tmpl, cfg)
			if !strings.Contains(out, tc.restart) {
				t.Errorf("want %q in rendered unit, got:\n%s", tc.restart, out)
			}
		})
	}
}

func TestSystemdTemplate_StartLimitTuning(t *testing.T) {
	cfg := SystemdConfig{
		SbmPath: "/usr/bin/sbm", DataDir: "/var/sbm", Port: "19090",
		LogPath: "/var/log/sbm", WorkingDir: "/var/sbm", HomeDir: "/root",
		KeepAlive: true, RunAtLoad: true,
	}
	for name, tmpl := range map[string]string{
		"system": systemdTemplate,
		"user":   systemdUserTemplate,
	} {
		t.Run(name, func(t *testing.T) {
			out := renderSystemd(t, tmpl, cfg)
			for _, want := range []string{"StartLimitIntervalSec=60", "StartLimitBurst=10", "RestartSec=5"} {
				if !strings.Contains(out, want) {
					t.Errorf("want %q in rendered unit, got:\n%s", want, out)
				}
			}
		})
	}
}
