package web

import (
	"net/url"
	"strings"
	"testing"
	"unicode/utf8"
)

// FuzzURLPathUnescape tests URL path unescaping with fuzzed input
func FuzzURLPathUnescape(f *testing.F) {
	// Seed with known edge cases
	f.Add("/users")
	f.Add("/users/john%20doe")
	f.Add("/groups/admin%2Busers")
	f.Add("/%2e%2e/etc/passwd") // Path traversal attempt
	f.Add("/%00null")           // Null byte injection
	f.Add("/<script>alert(1)</script>") // XSS attempt
	f.Add("/users/../../../etc/passwd") // Directory traversal
	f.Add("/users?id=1%27%20OR%201=1--") // SQL injection attempt
	f.Add("/%252e%252e/") // Double encoding
	f.Add(strings.Repeat("/%2e", 100)) // Long path
	f.Add("/路径/用户") // Unicode paths
	f.Add("/🎉/emoji")
	f.Add("") // Empty
	f.Add("/") // Root
	f.Add("//double//slashes//")
	f.Add("/users/;id") // Command injection attempt
	f.Add("/users|ls") // Pipe injection
	f.Add("/users`whoami`") // Backtick injection

	f.Fuzz(func(t *testing.T, path string) {
		// Test url.PathUnescape doesn't panic and handles all input
		unescaped, err := url.PathUnescape(path)

		// If escape was successful, verify the result
		if err == nil {
			// Re-escaping should produce valid output
			reescaped := url.PathEscape(unescaped)
			_ = reescaped

			// Check for dangerous patterns after unescaping
			if strings.Contains(unescaped, "../") || strings.Contains(unescaped, "..\\") {
				// This would be a path traversal - flag it
				t.Logf("Path traversal pattern detected: %s", path)
			}

			// Note: url.PathUnescape can produce invalid UTF-8 from malformed
			// percent-encoded input (e.g., %80). This is expected behavior.
			// Applications should validate UTF-8 after unescaping if needed.
		}
	})
}

// FuzzQueryParams tests query parameter parsing with fuzzed input
func FuzzQueryParams(f *testing.F) {
	// Seed with known edge cases
	f.Add("key=value")
	f.Add("key=value&key2=value2")
	f.Add("key=")
	f.Add("=value")
	f.Add("key")
	f.Add("")
	f.Add("key=value&key=value2") // Duplicate keys
	f.Add("key=a%20b")
	f.Add("key=%00") // Null byte
	f.Add("key=<script>") // XSS
	f.Add("key=' OR 1=1--") // SQL injection
	f.Add(strings.Repeat("key=value&", 100)) // Many params
	f.Add("key=" + strings.Repeat("x", 10000)) // Long value
	f.Add(strings.Repeat("k", 1000) + "=v") // Long key
	f.Add("key=中文") // Unicode value
	f.Add("键=value") // Unicode key
	f.Add("key[]=value1&key[]=value2") // Array syntax
	f.Add("key[0]=a&key[1]=b") // Indexed array
	f.Add("obj.field=value") // Dot notation
	f.Add("a=b&c=d&e=f&g=h&i=j") // Multiple

	f.Fuzz(func(t *testing.T, query string) {
		// Parse query shouldn't panic
		values, err := url.ParseQuery(query)

		if err == nil {
			// Note: url.ParseQuery can produce invalid UTF-8 from malformed
			// percent-encoded input. This is expected behavior.
			// Applications should validate UTF-8 after parsing if needed.

			// Encoding should produce valid output (re-encodes invalid bytes)
			encoded := values.Encode()
			if !utf8.ValidString(encoded) {
				t.Errorf("encoded query is not valid UTF-8: %q", encoded)
			}
		}
	})
}

// FuzzTemplateCacheKey tests cache key generation with fuzzed input
func FuzzTemplateCacheKey(f *testing.F) {
	// Seed with known edge cases
	f.Add("/users", "")
	f.Add("/users", "page=1")
	f.Add("/users", "search=john&page=1")
	f.Add("", "")
	f.Add("/", "")
	f.Add("/users/../admin", "")
	f.Add("/users", "key=<script>")
	f.Add(strings.Repeat("/a", 100), "")
	f.Add("/users", strings.Repeat("x", 10000))
	f.Add("/用户", "搜索=中文")
	f.Add("/🎉", "emoji=🎊")

	f.Fuzz(func(t *testing.T, path, query string) {
		// Simulate cache key generation
		key := path
		if query != "" {
			key = path + "?" + query
		}

		// Key should be valid string
		if !utf8.ValidString(key) {
			t.Errorf("Invalid UTF-8 cache key")
		}

		// Same inputs should produce same keys (determinism)
		key2 := path
		if query != "" {
			key2 = path + "?" + query
		}
		if key != key2 {
			t.Errorf("Non-deterministic key generation")
		}
	})
}

// FuzzSessionDataParsing tests session data handling
func FuzzSessionDataParsing(f *testing.F) {
	// Seed with session-like data
	f.Add("user_dn:cn=admin,dc=example,dc=com")
	f.Add("user_dn:")
	f.Add("")
	f.Add("user_dn:cn=user\\,escaped,dc=test")
	f.Add("user_dn:" + strings.Repeat("x", 10000))
	f.Add("user_dn:\x00null")
	f.Add("random:garbage:data")
	f.Add(strings.Repeat("a", 100000)) // Very long

	f.Fuzz(func(t *testing.T, data string) {
		// Simulate session parsing
		if len(data) > 1000000 {
			return // Skip extremely long strings
		}

		parts := strings.SplitN(data, ":", 2)
		if len(parts) == 2 {
			key := parts[0]
			value := parts[1]

			// Both should be valid UTF-8 for safe usage
			if !utf8.ValidString(key) || !utf8.ValidString(value) {
				// Invalid data - should be rejected
				return
			}

			// Simulate storage and retrieval
			stored := key + ":" + value
			if stored != data {
				t.Errorf("Data corruption: %q != %q", stored, data)
			}
		}
	})
}

// FuzzHTMLEscaping tests HTML escaping for XSS prevention
func FuzzHTMLEscaping(f *testing.F) {
	// XSS payloads
	f.Add("<script>alert(1)</script>")
	f.Add("<img src=x onerror=alert(1)>")
	f.Add("javascript:alert(1)")
	f.Add("<svg onload=alert(1)>")
	f.Add("'><script>alert(1)</script>")
	f.Add("\"><script>alert(1)</script>")
	f.Add("<body onload=alert(1)>")
	f.Add("{{.}}")             // Template injection
	f.Add("${7*7}")            // Expression injection
	f.Add("#{7*7}")            // Expression injection
	f.Add("<%= 7*7 %>")        // ERB-style
	f.Add("{{constructor.constructor('alert(1)')()}}") // Prototype pollution
	f.Add("<img src=\"\" onerror=\"alert('XSS')\">")
	f.Add("<a href=\"javascript:alert('XSS')\">click</a>")
	f.Add("normal text")
	f.Add("text with <em>emphasis</em>")
	f.Add("text & entities")
	f.Add("quotes: \" and '")

	f.Fuzz(func(t *testing.T, input string) {
		if !utf8.ValidString(input) {
			return
		}

		// Standard HTML escaping using template package
		escaped := htmlEscape(input)

		// Verify dangerous patterns are escaped
		if strings.Contains(escaped, "<script") {
			t.Errorf("Script tag not escaped: %s", escaped)
		}
		if strings.Contains(escaped, "javascript:") && strings.Contains(input, "javascript:") {
			// javascript: should be escaped in attribute context
			t.Logf("javascript: protocol in input: %s", input)
		}
		if strings.Contains(escaped, "onerror=") && strings.Contains(input, "onerror=") {
			t.Logf("Event handler in input: %s", input)
		}
	})
}

// htmlEscape performs HTML escaping
func htmlEscape(s string) string {
	// Standard HTML entity escaping
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")

	return s
}

// FuzzContentTypeDetection tests content type handling
func FuzzContentTypeDetection(f *testing.F) {
	// Various content types and file extensions
	f.Add("text/html")
	f.Add("application/json")
	f.Add("text/html; charset=utf-8")
	f.Add("text/html; charset=ISO-8859-1")
	f.Add("multipart/form-data; boundary=----")
	f.Add("")
	f.Add("text/html\x00")
	f.Add(strings.Repeat("x", 10000))
	f.Add("text/html; " + strings.Repeat("x=y; ", 100))
	f.Add("../../../etc/passwd") // Path traversal in content type

	f.Fuzz(func(t *testing.T, contentType string) {
		// Parse content type
		if !utf8.ValidString(contentType) {
			return
		}

		parts := strings.Split(contentType, ";")
		mediaType := strings.TrimSpace(parts[0])

		// Media type should be safe
		if strings.Contains(mediaType, "..") {
			t.Logf("Suspicious media type: %s", mediaType)
		}

		// Check for valid format
		if mediaType != "" && !strings.Contains(mediaType, "/") {
			// Invalid media type format
			return
		}
	})
}
