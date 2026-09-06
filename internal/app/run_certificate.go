package app

import (
	"errors"

	"github.com/AustinOyugi/no-oops-ops/internal/ingress"
)

func (a *App) runCertificate(args []string) error {
	if len(args) != 4 || args[0] != "import" {
		return errors.New("usage: noops certificate import <name> <certificate.pem> <private-key.pem>")
	}
	return a.ImportCertificate(args[1], args[2], args[3])
}

// ImportCertificate imports a certificate for ingress routes.
func (a *App) ImportCertificate(name, certificatePath, privateKeyPath string) error {
	return ingress.ImportCertificate(a.config, name, certificatePath, privateKeyPath)
}
