package app

import (
	"errors"

	"github.com/AustinOyugi/no-oops-ops/internal/ingress"
)

func (a *App) runCertificate(args []string) error {
	if len(args) != 4 || args[0] != "import" {
		return errors.New("usage: noops certificate import <name> <certificate.pem> <private-key.pem>")
	}
	return ingress.ImportCertificate(a.config, args[1], args[2], args[3])
}
