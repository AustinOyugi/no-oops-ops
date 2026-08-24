package ingress

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
)

var certificateName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// ImportCertificate stores a supplied certificate and private key in the
// platform data directory. The nginx service mounts this directory read-only.
func ImportCertificate(cfg config.Config, name, certificatePath, keyPath string) error {
	if !certificateName.MatchString(name) {
		return fmt.Errorf("certificate name %q must contain only lowercase letters, numbers, and hyphens", name)
	}
	certificate, err := os.ReadFile(certificatePath)
	if err != nil {
		return fmt.Errorf("read certificate %q: %w", certificatePath, err)
	}
	block, _ := pem.Decode(certificate)
	if block == nil || block.Type != "CERTIFICATE" {
		return fmt.Errorf("certificate %q does not contain a PEM certificate", certificatePath)
	}
	if _, err := x509.ParseCertificate(block.Bytes); err != nil {
		return fmt.Errorf("parse certificate %q: %w", certificatePath, err)
	}
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("read private key %q: %w", keyPath, err)
	}
	if block, _ := pem.Decode(key); block == nil || !regexp.MustCompile(`PRIVATE KEY$`).MatchString(block.Type) {
		return fmt.Errorf("private key %q does not contain a PEM private key", keyPath)
	}
	dir := filepath.Join(cfg.DataDir, "nginx", "certificates", name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create certificate directory %q: %w", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fullchain.pem"), certificate, 0o600); err != nil {
		return fmt.Errorf("write certificate %q: %w", name, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "privkey.pem"), key, 0o600); err != nil {
		return fmt.Errorf("write private key %q: %w", name, err)
	}
	return nil
}
