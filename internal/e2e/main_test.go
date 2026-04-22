//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ldap "github.com/netresearch/simple-ldap-go"
	"github.com/playwright-community/playwright-go"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/netresearch/ldap-manager/internal/options"
	"github.com/netresearch/ldap-manager/internal/web"
)

// Bootstrap constants shared between the pre-seeded OpenLDAP container and the
// in-process ldap-manager app under test.
const (
	bootstrapBaseDN    = "dc=example,dc=com"
	bootstrapDomain    = "example.com"
	bootstrapOrg       = "Example Inc"
	bootstrapAdminDN   = "cn=admin," + bootstrapBaseDN
	bootstrapAdminPass = "adminpassword"

	// Pre-seeded test user (see seedLDIF). uid is what LoginAsTestUser uses
	// because simple-ldap-go falls back from sAMAccountName to uid on OpenLDAP.
	bootstrapTestUser     = "testuser1"
	bootstrapTestPassword = "password1"
)

// aclLDIF loosens the osixia/openldap default access control so any
// authenticated user can read the directory. Without this, simple-ldap-go's
// FindUsers/FindGroups calls from a per-user bind fail with "No Such Object"
// because the default ACL hides the DIT root from non-admin users. The app
// renders the home page via per-user LDAP, so /users and /groups would
// return 500 without this loosening.
const aclLDIF = `dn: olcDatabase={1}mdb,cn=config
changetype: modify
replace: olcAccess
olcAccess: {0}to attrs=userPassword
  by self write
  by anonymous auth
  by * none
olcAccess: {1}to *
  by dn="cn=admin-user,ou=users,dc=example,dc=com" write
  by users read
  by * none
`

// seedLDIF is piped into ldapadd via the OpenLDAP container exec shell. It
// provisions the entries the e2e journeys exercise:
//
//   - ou=users,   ou=groups   (standard organisational units)
//   - uid=admin   in ou=users (the user E2E_ADMIN_USER=admin logs in as;
//     this entry is what indexHandler/FindUsers matches so the home page
//     renders without a 500)
//   - uid=testuser1 in ou=users (second user for list-visibility checks)
//   - cn=developers in ou=groups (non-empty group detail page)
//
// The osixia/openldap container also owns cn=admin,dc=example,dc=com (root
// DN, password=LDAP_ADMIN_PASSWORD) — that's what the app uses as its
// service account. Login form "admin" resolves via simple-ldap-go's
// uid-fallback to uid=admin,ou=users,dc=example,dc=com.
const seedLDIF = `dn: ou=users,dc=example,dc=com
objectClass: organizationalUnit
ou: users

dn: ou=groups,dc=example,dc=com
objectClass: organizationalUnit
ou: groups

dn: cn=admin-user,ou=users,dc=example,dc=com
objectClass: inetOrgPerson
objectClass: organizationalPerson
objectClass: person
objectClass: top
cn: admin-user
sn: Admin
uid: admin
mail: admin@example.com
userPassword: adminpassword
description: E2E admin user

dn: cn=testuser1,ou=users,dc=example,dc=com
objectClass: inetOrgPerson
objectClass: organizationalPerson
objectClass: person
objectClass: top
cn: testuser1
sn: User
uid: testuser1
mail: testuser1@example.com
userPassword: password1
description: E2E test user

dn: cn=developers,ou=groups,dc=example,dc=com
objectClass: groupOfNames
objectClass: top
cn: developers
member: cn=admin-user,ou=users,dc=example,dc=com
member: cn=testuser1,ou=users,dc=example,dc=com

dn: cn=viewers,ou=groups,dc=example,dc=com
objectClass: groupOfNames
objectClass: top
cn: viewers
description: Read-only observers (used by e2e add-to-group round-trip)
member: cn=admin-user,ou=users,dc=example,dc=com
`

// TestMain boots the e2e harness:
//
//  1. chdir to repo root so NewApp can load internal/web/static/manifest.json
//  2. start a pre-seeded OpenLDAP testcontainer
//  3. install Playwright browsers (no-op if already cached)
//  4. start ldap-manager in-process on 127.0.0.1:<random>, wait for /health/live
//  5. publish E2E_* env vars the per-test helpers pick up
//
// Any step failing aborts the whole suite with a clear message.
func TestMain(m *testing.M) {
	if err := chdirToRepoRoot(); err != nil {
		log.Fatalf("e2e bootstrap: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	ldapContainer, ldapURI, err := startOpenLDAP(ctx)
	if err != nil {
		log.Fatalf("e2e bootstrap: start openldap: %v", err)
	}
	defer func() {
		termCtx, termCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer termCancel()
		_ = ldapContainer.Terminate(termCtx)
	}()

	if err := seedOpenLDAP(ctx, ldapContainer); err != nil {
		log.Fatalf("e2e bootstrap: seed openldap: %v", err)
	}

	// Bulk-seed a large number of inetOrgPerson entries so diagnostic
	// tests (TestHeaderShrinkRepro) can observe flex behaviour under
	// realistic list length. The in-app cache picks these up on its
	// initial warm-up (which runs AFTER TestMain's seed phase).
	if err := bulkSeedUsers(ctx, ldapContainer, 200); err != nil {
		log.Fatalf("e2e bootstrap: bulk seed users: %v", err)
	}

	if err := playwright.Install(&playwright.RunOptions{Browsers: []string{"chromium"}}); err != nil {
		log.Fatalf("e2e bootstrap: install playwright: %v", err)
	}

	port, err := freePort()
	if err != nil {
		log.Fatalf("e2e bootstrap: free port: %v", err)
	}

	app, err := web.NewApp(&options.Opts{
		LDAP: ldap.Config{
			Server:            ldapURI,
			BaseDN:            bootstrapBaseDN,
			IsActiveDirectory: false,
		},
		ReadonlyUser:            bootstrapAdminDN,
		ReadonlyPassword:        bootstrapAdminPass,
		PersistSessions:         false,
		SessionDuration:         30 * time.Minute,
		CookieSecure:            false, // HTTP listener under test
		PoolMaxConnections:      5,
		PoolMinConnections:      1,
		PoolMaxIdleTime:         5 * time.Minute,
		PoolMaxLifetime:         30 * time.Minute,
		PoolHealthCheckInterval: 30 * time.Second,
		PoolConnectionTimeout:   10 * time.Second,
		PoolAcquireTimeout:      5 * time.Second,
	})
	if err != nil {
		log.Fatalf("e2e bootstrap: new app: %v", err)
	}

	appCtx, appCancel := context.WithCancel(context.Background())
	serverErr := make(chan error, 1)
	go func() { serverErr <- app.Listen(appCtx, fmt.Sprintf("127.0.0.1:%d", port)) }()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	if err := waitForReady(ctx, baseURL); err != nil {
		appCancel()
		log.Fatalf("e2e bootstrap: wait for app: %v", err)
	}

	_ = os.Setenv("E2E_BASE_URL", baseURL)
	_ = os.Setenv("E2E_ADMIN_USER", "admin")
	_ = os.Setenv("E2E_ADMIN_PASS", bootstrapAdminPass)
	_ = os.Setenv("E2E_TEST_USER", bootstrapTestUser)
	_ = os.Setenv("E2E_TEST_USER_PASS", bootstrapTestPassword)

	code := m.Run()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	_ = app.Shutdown(shutdownCtx)
	shutdownCancel()
	appCancel()
	select {
	case <-serverErr:
	case <-time.After(2 * time.Second):
	}

	os.Exit(code)
}

// startOpenLDAP brings up an osixia/openldap container on a random host port
// and returns its live ldap:// URI.
func startOpenLDAP(ctx context.Context) (testcontainers.Container, string, error) {
	req := testcontainers.ContainerRequest{
		Image:        "osixia/openldap:1.5.0",
		ExposedPorts: []string{"389/tcp"},
		Env: map[string]string{
			"LDAP_ORGANISATION":   bootstrapOrg,
			"LDAP_DOMAIN":         bootstrapDomain,
			"LDAP_ADMIN_PASSWORD": bootstrapAdminPass,
			"LDAP_TLS":            "false",
		},
		WaitingFor: wait.ForLog("slapd starting").WithStartupTimeout(90 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", fmt.Errorf("generic container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return container, "", fmt.Errorf("container host: %w", err)
	}
	// simple-ldap-go treats server URIs containing "localhost" (among other
	// substrings) as an example/test server and short-circuits real LDAP ops.
	// The testcontainers Docker provider returns "localhost" on native Linux;
	// rewrite it to 127.0.0.1 so the LDAP client actually talks to slapd.
	if host == "localhost" {
		host = "127.0.0.1"
	}
	port, err := container.MappedPort(ctx, "389")
	if err != nil {
		return container, "", fmt.Errorf("mapped port: %w", err)
	}

	return container, fmt.Sprintf("ldap://%s:%s", host, port.Port()), nil
}

// bulkSeedUsers appends N additional inetOrgPerson entries to ou=users so
// long-list / flex-pressure diagnostic tests have realistic data to work
// with. Idempotent because the LDIF uses unique cn=bulkuserNNN DNs.
func bulkSeedUsers(ctx context.Context, container testcontainers.Container, n int) error {
	var b strings.Builder
	for i := 0; i < n; i++ {
		cn := fmt.Sprintf("bulkuser%03d", i)
		fmt.Fprintf(&b, `dn: cn=%s,ou=users,dc=example,dc=com
objectClass: inetOrgPerson
objectClass: organizationalPerson
objectClass: person
objectClass: top
cn: %s
sn: Bulk
uid: %s
userPassword: pw

`, cn, cn, cn)
	}
	return execLDAP(ctx, container,
		"ldapadd", "-x", "-D", bootstrapAdminDN, "-w", bootstrapAdminPass,
		"-H", "ldap://localhost", b.String())
}

// seedOpenLDAP applies the ACL relaxation via ldapmodify (cn=config) and
// then pipes seedLDIF into ldapadd. Eventual consistency in the container
// means slapd may need a beat after "starting" before it accepts writes,
// so each step retries briefly.
func seedOpenLDAP(ctx context.Context, container testcontainers.Container) error {
	if err := execLDAP(ctx, container, "ldapmodify", "-Y", "EXTERNAL", "-H", "ldapi:///", aclLDIF); err != nil {
		return fmt.Errorf("apply ACL: %w", err)
	}
	if err := execLDAP(ctx, container, "ldapadd", "-x", "-D", bootstrapAdminDN, "-w", bootstrapAdminPass, "-H", "ldap://localhost", seedLDIF); err != nil {
		return fmt.Errorf("apply seed: %w", err)
	}
	return nil
}

// execLDAP pipes the given LDIF to the given ldap* utility inside the
// container, retrying up to 30s while slapd finishes its handshake.
func execLDAP(ctx context.Context, container testcontainers.Container, bin string, args ...string) error {
	// Last positional argument is the LDIF payload to pipe on stdin.
	ldif := args[len(args)-1]
	args = args[:len(args)-1]

	quoted := make([]string, 0, len(args))
	for _, a := range args {
		quoted = append(quoted, shellQuote(a))
	}

	cmd := []string{
		"bash", "-c",
		fmt.Sprintf(`%s %s <<'LDIF'
%sLDIF
`, bin, strings.Join(quoted, " "), ldif),
	}

	deadline := time.Now().Add(30 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		exitCode, _, execErr := container.Exec(ctx, cmd)
		if execErr == nil && exitCode == 0 {
			return nil
		}
		lastErr = fmt.Errorf("%s exit=%d err=%v", bin, exitCode, execErr)
		time.Sleep(500 * time.Millisecond)
	}
	return lastErr
}

// shellQuote is a minimal POSIX-safe single-quote wrapper.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// waitForReady polls /health/live until 200 OK or ctx deadline.
func waitForReady(ctx context.Context, baseURL string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(30 * time.Second)
	url := baseURL + "/health/live"

	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("ldap-manager did not become ready at %s", baseURL)
}

// freePort grabs an ephemeral localhost port from the kernel.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// chdirToRepoRoot walks up from the current directory until it finds go.mod.
// Matches the helper used by internal/web tests; NewApp resolves the asset
// manifest relative to the process cwd.
func chdirToRepoRoot() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	for dir := cwd; dir != "/" && dir != "."; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return os.Chdir(dir)
		}
	}
	return fmt.Errorf("could not locate repo root from %s", cwd)
}
