package app

import (
	"github.com/AustinOyugi/no-oops-ops/internal/ingress"
)

// ImportCertificate imports a certificate for ingress routes.
func (a *App) ImportCertificate(name, certificatePath, privateKeyPath string) error {
	return ingress.ImportCertificate(a.config, name, certificatePath, privateKeyPath)
}
