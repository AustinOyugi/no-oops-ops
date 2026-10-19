package ingress

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

func writeTestCertificate(t *testing.T, s *Service, primary string, domains []string, expired bool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	until := time.Now().Add(time.Hour)
	if expired {
		until = time.Now().Add(-time.Minute)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: domains, NotBefore: time.Now().Add(-time.Hour), NotAfter: until}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.certificateDir(), "live", primary, "fullchain.pem")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestCertificatesCoverAllAliasesAndSharedPaths(t *testing.T) {
	runner := &recordingRunner{}
	s := &Service{logger: slog.Default(), runner: runner, config: config.Config{DataDir: t.TempDir(), ACMEEmail: "ops@example.test"}}
	routes := []Route{
		{Domain: "primary.example.test", Domains: []string{"primary.example.test", "second.example.test", "second.example.test"}, TLS: true},
		{Domain: "primary.example.test", Domains: []string{"third.example.test"}, PathPrefix: "/api", TLS: true},
	}
	issued, err := s.issueMissingCertificates(context.Background(), routes)
	if err != nil || !issued || len(runner.calls) != 1 {
		t.Fatalf("issued=%v calls=%v err=%v", issued, runner.calls, err)
	}
	call := strings.Join(runner.calls[0], " ")
	for _, want := range []string{"--cert-name primary.example.test", "--domains primary.example.test,second.example.test,third.example.test"} {
		if !strings.Contains(call, want) {
			t.Fatalf("missing %q: %s", want, call)
		}
	}
	if strings.Contains(call, "--force-renewal") {
		t.Fatalf("forced renewal of new certificate: %s", call)
	}
}

func TestExistingCertificateCoverageControlsReissue(t *testing.T) {
	for _, tc := range []struct {
		name             string
		domains          []string
		expired, reissue bool
	}{
		{name: "missing alias", domains: []string{"primary.example.test"}, reissue: true},
		{name: "all names", domains: []string{"primary.example.test", "alias.example.test"}},
		{name: "additional SAN retained", domains: []string{"primary.example.test", "alias.example.test", "old.example.test"}},
		{name: "expired", domains: []string{"primary.example.test", "alias.example.test"}, expired: true, reissue: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &recordingRunner{}
			s := &Service{logger: slog.Default(), runner: runner, config: config.Config{DataDir: t.TempDir(), ACMEEmail: "ops@example.test"}}
			writeTestCertificate(t, s, "primary.example.test", tc.domains, tc.expired)
			routes := []Route{{Domain: "primary.example.test", Domains: []string{"primary.example.test", "alias.example.test"}, TLS: true}}
			available := s.routesWithAvailableCertificates(routes)
			if available[0].TLS == tc.reissue {
				t.Fatalf("TLS availability=%v reissue=%v", available[0].TLS, tc.reissue)
			}
			if !routes[0].TLS {
				t.Fatal("modified saved route")
			}
			issued, err := s.issueMissingCertificates(context.Background(), routes)
			if err != nil || issued != tc.reissue {
				t.Fatalf("issued=%v err=%v", issued, err)
			}
			if tc.reissue {
				if len(runner.calls) != 1 || !strings.Contains(strings.Join(runner.calls[0], " "), "--force-renewal") {
					t.Fatalf("existing lineage not repaired: %v", runner.calls)
				}
			} else if len(runner.calls) != 0 {
				t.Fatalf("unnecessary issuance: %v", runner.calls)
			}
		})
	}
}

func TestACMERejectsWildcardAliasBeforeIssuance(t *testing.T) {
	runner := &recordingRunner{}
	s := &Service{logger: slog.Default(), runner: runner, config: config.Config{DataDir: t.TempDir(), ACMEEmail: "ops@example.test"}}
	_, err := s.issueMissingCertificates(context.Background(), []Route{{Domain: "primary.example.test", Domains: []string{"*.example.test"}, TLS: true}})
	if err == nil || !strings.Contains(err.Error(), "wildcard") || len(runner.calls) != 0 {
		t.Fatalf("calls=%v err=%v", runner.calls, err)
	}
}

func TestACMESkipsImportedAndHTTPRoutes(t *testing.T) {
	runner := &recordingRunner{}
	s := &Service{logger: slog.Default(), runner: runner, config: config.Config{DataDir: t.TempDir()}}
	issued, err := s.issueMissingCertificates(context.Background(), []Route{
		{Domain: "example.test", Domains: []string{"*.example.test"}, TLS: true, TLSCertificate: "origin"},
		{Domain: "http.example.test"},
	})
	if err != nil || issued || len(runner.calls) != 0 {
		t.Fatalf("issued=%v calls=%v err=%v", issued, runner.calls, err)
	}
}

func TestRenderedHostnamesMatchCertificateSANs(t *testing.T) {
	routes := []Route{
		{Environment: "dev", App: "app", Domain: "primary.example.test", Domains: []string{"primary.example.test", "alias.example.test", "alias.example.test"}, Service: "app", Port: 8080, TLS: true, PathPrefix: "/"},
		{Environment: "dev", App: "api", Domain: "primary.example.test", Domains: []string{"third.example.test"}, Service: "api", Port: 8080, TLS: true, PathPrefix: "/api"},
	}
	expected := []string{"primary.example.test", "alias.example.test", "third.example.test"}
	if names := certificateDomains("primary.example.test", routes); !reflect.DeepEqual(names, expected) {
		t.Fatalf("domains=%v", names)
	}
	rendered, err := RenderConfig(routes)
	if err != nil {
		t.Fatal(err)
	}
	want := "server_name " + strings.Join(expected, " ") + ";"
	if strings.Count(string(rendered), want) != 2 {
		t.Fatalf("nginx hosts differ from SAN request:\n%s", rendered)
	}
	files, err := RenderFiles(routes)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(files["external/primary-example-test.conf"]), want) != 2 {
		t.Fatal("split nginx configuration has different hostnames")
	}
}

type certificateRepairRunner struct {
	reloadRecordingRunner
	t         *testing.T
	service   *Service
	issuances int
}

func (r *certificateRepairRunner) Run(ctx context.Context, name string, args []string, options command.RunOptions) (command.Result, error) {
	if len(args) > 0 && args[0] == "run" && strings.Contains(strings.Join(args, " "), "certbot/certbot") {
		r.issuances++
		pending, err := os.ReadFile(filepath.Join(r.service.ingressDir(), "nginx.conf"))
		if err != nil {
			r.t.Fatal(err)
		}
		if strings.Contains(string(pending), "listen 443 ssl") {
			r.t.Fatal("enabled HTTPS with certificate missing alias before repair")
		}
		if !strings.Contains(string(pending), "primary.example.test alias.example.test") {
			r.t.Fatal("alias not available for HTTP challenge")
		}
		writeTestCertificate(r.t, r.service, "primary.example.test", []string{"primary.example.test", "alias.example.test"}, false)
		return command.Result{}, nil
	}
	return r.reloadRecordingRunner.Run(ctx, name, args, options)
}

func TestReconcileRepairsExistingAliasCertificateAndThenReusesIt(t *testing.T) {
	root := t.TempDir()
	runner := &certificateRepairRunner{t: t, reloadRecordingRunner: reloadRecordingRunner{output: "nginx-one"}}
	s := &Service{logger: slog.Default(), runner: runner, config: config.Config{StateDir: root, DataDir: root, ACMEEmail: "ops@example.test", NginxName: "nginx"}}
	runner.service = s
	if err := os.MkdirAll(s.configDir(), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.ingressDir(), "nginx.conf"), []byte("existing nginx config"), 0600); err != nil {
		t.Fatal(err)
	}
	writeTestCertificate(t, s, "primary.example.test", []string{"primary.example.test"}, false)
	m := manifest.Manifest{Name: "lango", Service: manifest.Service{InternalPort: 8080}, Expose: manifest.Expose{Enabled: true, TLS: true, Domain: "primary.example.test", Domains: []string{"primary.example.test", "alias.example.test"}, PathPrefix: "/"}}
	existing, _, err := updateRoute(nil, "dev", m, "lango_app")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.writeRoutes(existing); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := s.Reconcile(context.Background(), "dev", m, "lango_app"); err != nil {
			t.Fatal(err)
		}
	}
	if runner.issuances != 1 {
		t.Fatalf("issuances=%d, want one repair", runner.issuances)
	}
	content, err := os.ReadFile(filepath.Join(s.ingressDir(), "nginx.conf"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"listen 443 ssl", "server_name primary.example.test alias.example.test;", "/live/primary.example.test/fullchain.pem"} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("missing %q after repair", want)
		}
	}
}
