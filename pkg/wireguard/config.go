package wireguard

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

// WireGuardConfig represents a WireGuard configuration
type Config struct {
	Interface InterfaceConfig
	Peers     []PeerConfig
}

// InterfaceConfig represents WireGuard interface configuration
type InterfaceConfig struct {
	Name       string
	Address    string
	PrivateKey string
	ListenPort int
	MTU        int
	DNS        []string
}

// PeerConfig represents WireGuard peer configuration
type PeerConfig struct {
	PublicKey           string
	AllowedIPs          []string
	Endpoint            string
	PersistentKeepalive int
}

// ConfigTemplate is the WireGuard config template
const ConfigTemplate = `[Interface]
PrivateKey = {{ .Interface.PrivateKey }}
Address = {{ .Interface.Address }}
{{- if .Interface.ListenPort }}
ListenPort = {{ .Interface.ListenPort }}
{{- end }}
{{- if .Interface.MTU }}
MTU = {{ .Interface.MTU }}
{{- end }}
{{- if .Interface.DNS }}
DNS = {{ join .Interface.DNS ", " }}
{{- end }}

{{- range .Peers }}

[Peer]
PublicKey = {{ .PublicKey }}
AllowedIPs = {{ join .AllowedIPs ", " }}
{{- if .Endpoint }}
Endpoint = {{ .Endpoint }}
{{- end }}
{{- if .PersistentKeepalive }}
PersistentKeepalive = {{ .PersistentKeepalive }}
{{- end }}
{{- end }}
`

// GenerateConfig generates a WireGuard configuration file
func GenerateConfig(config *Config) (string, error) {
	funcMap := template.FuncMap{
		"join": func(elems []string, sep string) string {
			return fmt.Sprintf("%v", elems)
		},
	}

	tmpl, err := template.New("wg").Funcs(funcMap).Parse(ConfigTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, config)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// WriteConfig writes WireGuard config to file
func WriteConfig(config *Config, path string) error {
	content, err := GenerateConfig(config)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// StartInterface starts a WireGuard interface
func StartInterface(name string) error {
	cmd := exec.Command("wg-quick", "up", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start interface %s: %s: %w", name, string(output), err)
	}
	return nil
}

// StopInterface stops a WireGuard interface
func StopInterface(name string) error {
	cmd := exec.Command("wg-quick", "down", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stop interface %s: %s: %w", name, string(output), err)
	}
	return nil
}

// GetInterfaceStatus gets WireGuard interface status
func GetInterfaceStatus(name string) (string, error) {
	cmd := exec.Command("wg", "show", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get status for %s: %s: %w", name, string(output), err)
	}
	return string(output), nil
}

// GenerateKeyPair generates a WireGuard key pair
func GenerateKeyPair() (privateKey, publicKey string, err error) {
	// Generate private key
	cmd := exec.Command("wg", "genkey")
	privateKeyBytes, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}
	privateKey = string(privateKeyBytes)[:len(privateKeyBytes)-1] // Remove newline

	// Generate public key
	cmd = exec.Command("wg", "pubkey")
	cmd.Stdin = bytes.NewReader(privateKeyBytes)
	publicKeyBytes, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate public key: %w", err)
	}
	publicKey = string(publicKeyBytes)[:len(publicKeyBytes)-1] // Remove newline

	return privateKey, publicKey, nil
}

// GeneratePSK generates a WireGuard preshared key
func GeneratePSK() (string, error) {
	cmd := exec.Command("wg", "genpsk")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to generate PSK: %w", err)
	}
	return string(output)[:len(output)-1], nil // Remove newline
}
