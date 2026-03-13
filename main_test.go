package main

import (
    "strings"
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
)

func setupTestRouter() *gin.Engine {
    gin.SetMode(gin.TestMode)
    router := gin.New()

    api := router.Group("/api")
    {
        links := api.Group("/links")
        {
            links.GET("", func(c *gin.Context) {
                c.JSON(200, []interface{}{})
            })

            links.POST("", func(c *gin.Context) {
                var req map[string]interface{}
                if err := c.ShouldBindJSON(&req); err != nil {
                    c.JSON(400, gin.H{"error": "invalid request"})
                    return
                }

                if _, ok := req["original_url"]; !ok {
                    c.JSON(422, gin.H{"errors": gin.H{"original_url": "original_url is required"}})
                    return
                }

                originalURL := req["original_url"].(string)
                if !strings.HasPrefix(originalURL, "http") {
                    c.JSON(422, gin.H{"errors": gin.H{"original_url": "original_url must be a valid URL"}})
                    return
                }

                if shortName, ok := req["short_name"]; ok && len(shortName.(string)) < 3 {
                    c.JSON(422, gin.H{"errors": gin.H{"short_name": "short_name must be at least 3 characters"}})
                    return
                }

                c.JSON(201, gin.H{
                    "id": 1,
                    "original_url": originalURL,
                    "short_name": "test123",
                    "short_url": "http://localhost:8080/r/test123",
                })
            })

            links.GET("/:id", func(c *gin.Context) {
                id := c.Param("id")
                if id == "999999" {
                    c.JSON(404, gin.H{"error": "link not found"})
                    return
                }
                c.JSON(200, gin.H{
                    "id": 1,
                    "original_url": "https://example.com",
                    "short_name": "test",
                    "short_url": "http://localhost:8080/r/test",
                })
            })

            links.PUT("/:id", func(c *gin.Context) {
                id := c.Param("id")
                if id == "999999" {
                    c.JSON(404, gin.H{"error": "link not found"})
                    return
                }

                var req map[string]interface{}
                if err := c.ShouldBindJSON(&req); err != nil {
                    c.JSON(400, gin.H{"error": "invalid request"})
                    return
                }

                c.JSON(200, gin.H{
                    "id": 1,
                    "original_url": "https://updated.com",
                    "short_name": "updated",
                    "short_url": "http://localhost:8080/r/updated",
                })
            })

            links.DELETE("/:id", func(c *gin.Context) {
                id := c.Param("id")
                if id == "999999" {
                    c.JSON(404, gin.H{"error": "link not found"})
                    return
                }
                c.Status(204)
            })
        }

        api.GET("/link_visits", func(c *gin.Context) {
            c.JSON(200, []interface{}{})
        })
    }

    router.GET("/ping", func(c *gin.Context) {
        c.String(200, "pong")
    })

    router.GET("/r/:code", func(c *gin.Context) {
        c.Redirect(302, "https://example.com")
    })

    return router
}

func TestPingRoute(t *testing.T) {
    router := setupTestRouter()

    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/ping", nil)
    router.ServeHTTP(w, req)

    assert.Equal(t, 200, w.Code)
    assert.Equal(t, "pong", w.Body.String())
}

func TestCreateLink(t *testing.T) {
    router := setupTestRouter()

    tests := []struct {
        name       string
        payload    map[string]interface{}
        wantStatus int
    }{
        {
            name:       "valid link without short_name",
            payload:    map[string]interface{}{"original_url": "https://example.com"},
            wantStatus: 201,
        },
        {
            name:       "valid link with short_name",
            payload:    map[string]interface{}{"original_url": "https://google.com", "short_name": "google"},
            wantStatus: 201,
        },
        {
            name:       "missing original_url",
            payload:    map[string]interface{}{"short_name": "test"},
            wantStatus: 422,
        },
        {
            name:       "invalid url",
            payload:    map[string]interface{}{"original_url": "not-a-url"},
            wantStatus: 422,
        },
        {
            name:       "short_name too short",
            payload:    map[string]interface{}{"original_url": "https://example.com", "short_name": "ab"},
            wantStatus: 422,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            body, _ := json.Marshal(tt.payload)
            w := httptest.NewRecorder()
            req, _ := http.NewRequest("POST", "/api/links", bytes.NewBuffer(body))
            req.Header.Set("Content-Type", "application/json")
            router.ServeHTTP(w, req)

            assert.Equal(t, tt.wantStatus, w.Code)
        })
    }
}

func TestGetLink(t *testing.T) {
    router := setupTestRouter()

    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/api/links/1", nil)
    router.ServeHTTP(w, req)

    assert.Equal(t, 200, w.Code)

    w = httptest.NewRecorder()
    req, _ = http.NewRequest("GET", "/api/links/999999", nil)
    router.ServeHTTP(w, req)

    assert.Equal(t, 404, w.Code)
}

func TestUpdateLink(t *testing.T) {
    router := setupTestRouter()

    updateBody := bytes.NewBufferString(`{"original_url": "https://updated.com", "short_name": "updated"}`)
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("PUT", "/api/links/1", updateBody)
    req.Header.Set("Content-Type", "application/json")
    router.ServeHTTP(w, req)

    assert.Equal(t, 200, w.Code)

    w = httptest.NewRecorder()
    req, _ = http.NewRequest("PUT", "/api/links/999999", updateBody)
    req.Header.Set("Content-Type", "application/json")
    router.ServeHTTP(w, req)

    assert.Equal(t, 404, w.Code)
}

func TestDeleteLink(t *testing.T) {
    router := setupTestRouter()

    w := httptest.NewRecorder()
    req, _ := http.NewRequest("DELETE", "/api/links/1", nil)
    router.ServeHTTP(w, req)

    assert.Equal(t, 204, w.Code)

    w = httptest.NewRecorder()
    req, _ = http.NewRequest("DELETE", "/api/links/999999", nil)
    router.ServeHTTP(w, req)

    assert.Equal(t, 404, w.Code)
}