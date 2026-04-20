// internal/web/search_index_test.go
package web

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchIndex_ShapeAndContentType(t *testing.T) {
	app, _ := setupFullTestApp(t)

	req := httptest.NewRequest("GET", "/api/search-index.json", nil)
	resp, err := app.fiber.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	assert.NotEmpty(t, resp.Header.Get("ETag"))

	body, _ := io.ReadAll(resp.Body)
	var entries []SearchIndexEntry
	require.NoError(t, json.Unmarshal(body, &entries))

	for _, e := range entries {
		assert.Contains(t, []string{"user", "group", "computer"}, e.Type)
		assert.NotEmpty(t, e.DN)
		assert.NotEmpty(t, e.CN)
	}
}

func TestSearchIndex_ETagRespected(t *testing.T) {
	app, _ := setupFullTestApp(t)

	resp1, err := app.fiber.Test(httptest.NewRequest("GET", "/api/search-index.json", nil))
	require.NoError(t, err)
	etag := resp1.Header.Get("ETag")
	require.NotEmpty(t, etag)
	_ = resp1.Body.Close()

	req2 := httptest.NewRequest("GET", "/api/search-index.json", nil)
	req2.Header.Set("If-None-Match", etag)
	resp2, err := app.fiber.Test(req2)
	require.NoError(t, err)
	defer resp2.Body.Close()

	assert.Equal(t, fiber.StatusNotModified, resp2.StatusCode)
}
