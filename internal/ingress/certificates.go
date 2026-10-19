package ingress

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"sort"
	"time"
)

// certificateDomains is shared by nginx rendering and ACME issuance: every
// hostname on a virtual host must be covered by the certificate it serves.
func certificateDomains(primary string, routes []Route) []string {
	names := []string{primary}
	seen := map[string]bool{primary: true}
	for _, route := range routes {
		if route.Domain != primary {
			continue
		}
		for _, domain := range route.Domains {
			if domain != "" && !seen[domain] {
				names = append(names, domain)
				seen[domain] = true
			}
		}
	}
	sort.Strings(names[1:])
	return names
}

func certificateCoverage(path string, domains []string) (exists, covers bool, err error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return true, false, fmt.Errorf("invalid PEM certificate in %q", path)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return true, false, fmt.Errorf("parse certificate %q: %w", path, err)
	}
	now := time.Now()
	if now.Before(cert.NotBefore) || !now.Before(cert.NotAfter) {
		return true, false, nil
	}
	for _, domain := range domains {
		if cert.VerifyHostname(domain) != nil {
			return true, false, nil
		}
	}
	return true, true, nil
}
