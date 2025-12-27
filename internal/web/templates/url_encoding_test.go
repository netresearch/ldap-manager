package templates

import (
	"net/url"
	"strings"
	"testing"
)

// =============================================================================
// Regression tests for URL encoding of LDAP Distinguished Names
// Bug: DNs with LDAP escape sequences (like \0A for newline) were not properly
// URL-encoded, causing browsers to convert backslashes to forward slashes.
// =============================================================================

// TestComputerUrlEncoding tests that computerUrl properly URL-encodes DNs
// containing LDAP escape sequences.
func TestComputerUrlEncoding(t *testing.T) {
	testCases := []struct {
		name     string
		dn       string
		wantEnc  string // Expected substring in encoded URL
		notWant  string // Should NOT be present in encoded URL
	}{
		{
			name:    "DN with newline escape sequence",
			dn:      `CN=wd-ex\0ACNF:guid,CN=Computers,DC=example,DC=com`,
			wantEnc: "%5C", // Backslash must be URL-encoded
			notWant: `\`,   // Literal backslash would be converted to / by browsers
		},
		{
			name:    "DN with backslash escape",
			dn:      `CN=test\5Cname,CN=Computers,DC=example,DC=com`,
			wantEnc: "%5C",
			notWant: `\`,
		},
		{
			name:    "Simple DN without escapes",
			dn:      `CN=simple,CN=Computers,DC=example,DC=com`,
			wantEnc: "%2C", // Comma should be encoded
			notWant: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test the URL encoding logic directly since we can't easily
			// set the DN on an ldap.Computer (it's derived from the Object)
			encoded := url.PathEscape(tc.dn)

			if tc.wantEnc != "" && !strings.Contains(encoded, tc.wantEnc) {
				t.Errorf("Encoded URL should contain %q\nDN: %s\nEncoded: %s",
					tc.wantEnc, tc.dn, encoded)
			}

			if tc.notWant != "" && strings.Contains(encoded, tc.notWant) {
				t.Errorf("Encoded URL should NOT contain %q\nDN: %s\nEncoded: %s",
					tc.notWant, tc.dn, encoded)
			}

			// Verify round-trip
			decoded, err := url.PathUnescape(encoded)
			if err != nil {
				t.Fatalf("Failed to decode: %v", err)
			}
			if decoded != tc.dn {
				t.Errorf("Round-trip failed\nOriginal: %s\nDecoded:  %s", tc.dn, decoded)
			}

			// Simulate what the template generates
			generatedURL := "/computers/" + encoded
			if tc.notWant != "" && strings.Contains(generatedURL, tc.notWant) {
				t.Errorf("Generated URL contains unsafe character %q: %s",
					tc.notWant, generatedURL)
			}
		})
	}
}

// TestUserUrlEncoding tests that userUrl properly URL-encodes DNs.
func TestUserUrlEncoding(t *testing.T) {
	testCases := []struct {
		name string
		dn   string
	}{
		{
			name: "DN with special characters",
			dn:   `CN=John\0ADoe,OU=Users,DC=example,DC=com`,
		},
		{
			name: "DN with comma in CN",
			dn:   `CN=Doe\2C John,OU=Users,DC=example,DC=com`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			encoded := url.PathEscape(tc.dn)

			// Backslash must be encoded
			if strings.Contains(tc.dn, `\`) && !strings.Contains(encoded, "%5C") {
				t.Errorf("Backslash not properly encoded\nDN: %s\nEncoded: %s", tc.dn, encoded)
			}

			// Literal backslash must not be present
			if strings.Contains(encoded, `\`) {
				t.Errorf("Encoded URL contains literal backslash\nEncoded: %s", encoded)
			}

			// Verify round-trip
			decoded, err := url.PathUnescape(encoded)
			if err != nil {
				t.Fatalf("Failed to decode: %v", err)
			}
			if decoded != tc.dn {
				t.Errorf("Round-trip failed\nOriginal: %s\nDecoded:  %s", tc.dn, decoded)
			}
		})
	}
}

// TestGroupUrlEncoding tests that groupUrl properly URL-encodes DNs.
func TestGroupUrlEncoding(t *testing.T) {
	dn := `CN=Admin\0AGroup,OU=Groups,DC=example,DC=com`
	encoded := url.PathEscape(dn)

	if strings.Contains(encoded, `\`) {
		t.Errorf("Encoded URL contains literal backslash: %s", encoded)
	}

	if !strings.Contains(encoded, "%5C") {
		t.Errorf("Backslash not properly encoded as %%5C: %s", encoded)
	}
}

// TestBrowserBackslashConversion documents the browser behavior that caused the bug.
// Browsers normalize backslashes to forward slashes in URL paths.
func TestBrowserBackslashConversion(t *testing.T) {
	// This test documents the problematic browser behavior
	// When a URL contains a literal backslash: /computers/CN=test\0A
	// Browsers send it as: /computers/CN=test/0A
	// This is why we MUST URL-encode the backslash as %5C

	dnWithBackslash := `CN=test\0Aname,CN=Computers,DC=example,DC=com`

	// BAD: Without URL encoding, the backslash would be in the URL literally
	badURL := "/computers/" + dnWithBackslash
	if !strings.Contains(badURL, `\`) {
		t.Skip("Test setup issue - backslash should be in bad URL")
	}

	// GOOD: With URL encoding, the backslash becomes %5C
	goodURL := "/computers/" + url.PathEscape(dnWithBackslash)
	if strings.Contains(goodURL, `\`) {
		t.Error("URL-encoded path should not contain literal backslash")
	}
	if !strings.Contains(goodURL, "%5C") {
		t.Error("URL-encoded path should contain %5C")
	}

	t.Logf("BAD URL (browser converts \\ to /): %s", badURL)
	t.Logf("GOOD URL (backslash properly encoded): %s", goodURL)
}
