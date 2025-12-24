//go:build e2e

// Package e2e provides end-to-end browser tests using Playwright.
// These tests require a running LDAP Manager instance and browser.
//
// Run with: go test -tags=e2e ./internal/e2e/...
//
// Prerequisites:
//   - Install Playwright browsers: go run github.com/playwright-community/playwright-go/cmd/playwright install chromium
//   - Running LDAP Manager instance (default: http://localhost:8080)
//   - Running LDAP server for authentication
package e2e
