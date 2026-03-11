package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"code/db"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

type LinkHandler struct {
	queries *db.Queries
	db      *sql.DB
	baseURL string
}

func NewLinkHandler(dbConn *sql.DB) *LinkHandler {
	queries := db.New(dbConn)
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	return &LinkHandler{
		queries: queries,
		db:      dbConn,
		baseURL: baseURL,
	}
}

type CreateLinkRequest struct {
	OriginalURL string `json:"original_url" binding:"required"`
	ShortName   string `json:"short_name"`
}

func generateShortName() string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, 6)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func (h *LinkHandler) CreateLink(c *gin.Context) {
	var req CreateLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	shortName := req.ShortName
	if shortName == "" {
		shortName = generateShortName()
	}

	shortURL := fmt.Sprintf("%s/r/%s", h.baseURL, shortName)

	link, err := h.queries.CreateLink(c, db.CreateLinkParams{
		OriginalUrl: req.OriginalURL,
		ShortName:   shortName,
		ShortUrl:    shortURL,
	})

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			c.JSON(http.StatusConflict, gin.H{"error": "short name already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, link)
}

func (h *LinkHandler) GetLink(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	link, err := h.queries.GetLink(c, int32(id))
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, link)
}

func (h *LinkHandler) ListLinks(c *gin.Context) {
    rangeParam := c.Query("range")

    var start, end int
    var err error

    if rangeParam != "" {
        rangeStr := strings.Trim(rangeParam, "[]")
        parts := strings.Split(rangeStr, ",")

        if len(parts) == 2 {
            start, err = strconv.Atoi(strings.TrimSpace(parts[0]))
            if err != nil {
                c.JSON(http.StatusBadRequest, gin.H{"error": "invalid range format"})
                return
            }

            end, err = strconv.Atoi(strings.TrimSpace(parts[1]))
            if err != nil {
                c.JSON(http.StatusBadRequest, gin.H{"error": "invalid range format"})
                return
            }

            if start < 0 || end < start {
                c.JSON(http.StatusBadRequest, gin.H{"error": "invalid range values"})
                return
            }
        } else {
            c.JSON(http.StatusBadRequest, gin.H{"error": "invalid range format"})
            return
        }
    } else {
        start = 0
        end = 9
    }

    limit := end - start + 1
    offset := start

    total, err := h.queries.CountLinks(c)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    links, err := h.queries.ListLinks(c, db.ListLinksParams{
        Limit:  int32(limit),
        Offset: int32(offset),
    })
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    contentRange := fmt.Sprintf("links %d-%d/%d", start, start+len(links)-1, total)
    c.Header("Content-Range", contentRange)

    c.JSON(http.StatusOK, links)
}

func (h *LinkHandler) UpdateLink(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req CreateLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	shortURL := fmt.Sprintf("%s/r/%s", h.baseURL, req.ShortName)

	link, err := h.queries.UpdateLink(c, db.UpdateLinkParams{
		ID:          int32(id),
		OriginalUrl: req.OriginalURL,
		ShortName:   req.ShortName,
		ShortUrl:    shortURL,
	})

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, link)
}

func (h *LinkHandler) DeleteLink(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	err = h.queries.DeleteLink(c, int32(id))
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

func main() {
	dbConn, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer dbConn.Close()

	if err := dbConn.Ping(); err != nil {
		log.Fatal("Failed to ping database:", err)
	}

	log.Println("Connected to database")

	router := gin.Default()

	linkHandler := NewLinkHandler(dbConn)

	api := router.Group("/api")
    {
        links := api.Group("/links")
        {
            links.GET("", linkHandler.ListLinks)
            links.POST("", linkHandler.CreateLink)
            links.GET("/:id", linkHandler.GetLink)
            links.PUT("/:id", linkHandler.UpdateLink)
            links.DELETE("/:id", linkHandler.DeleteLink)
        }
    }

	router.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	router.Run(":" + port)
}