package main

import (
    "context"
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
	"github.com/go-playground/validator/v10"
)

type ValidationError struct {
    Field string `json:"field"`
    Error string `json:"error"`
}

type ValidationErrors struct {
    Errors map[string]string `json:"errors"`
}

var validate = validator.New()

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
    OriginalURL string `json:"original_url" validate:"required,url"`
    ShortName   string `json:"short_name" validate:"omitempty,min=3,max=32"`
}

func generateShortName() string {
    const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    rng := rand.New(rand.NewSource(time.Now().UnixNano()))
    b := make([]byte, 6)
    for i := range b {
        b[i] = letters[rng.Intn(len(letters))]
    }
    return string(b)
}

func (h *LinkHandler) CreateLink(c *gin.Context) {
    var req CreateLinkRequest

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
        return
    }

    if err := validate.Struct(req); err != nil {
        c.JSON(http.StatusUnprocessableEntity, FormatValidationError(err))
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
        if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "23505") {
            c.JSON(http.StatusUnprocessableEntity, ValidationErrors{
                Errors: map[string]string{
                    "short_name": "short name already in use",
                },
            })
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
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
        return
    }

    // Валидируем данные
    if err := validate.Struct(req); err != nil {
        c.JSON(http.StatusUnprocessableEntity, FormatValidationError(err))
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
        if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "23505") {
            c.JSON(http.StatusUnprocessableEntity, ValidationErrors{
                Errors: map[string]string{
                    "short_name": "short name already in use",
                },
            })
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

	_, err = h.queries.GetLink(c, int32(id))
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	err = h.queries.DeleteLink(c, int32(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *LinkHandler) RedirectHandler(c *gin.Context) {
    shortName := c.Param("code")

    link, err := h.queries.GetLinkByShortName(c, shortName)
    if err != nil {
        if err == sql.ErrNoRows {
            c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    ip := c.ClientIP()
    userAgent := c.Request.UserAgent()
    referer := c.Request.Referer()
    status := http.StatusFound

    go func() {
        _, err := h.queries.CreateLinkVisit(context.Background(), db.CreateLinkVisitParams{
            LinkID:    link.ID,
            Ip:        sql.NullString{String: ip, Valid: ip != ""},
            UserAgent: sql.NullString{String: userAgent, Valid: userAgent != ""},
            Referer:   sql.NullString{String: referer, Valid: referer != ""},
            Status:    int32(status),
        })
        if err != nil {
            log.Printf("Failed to save visit: %v", err)
        }
    }()

    c.Redirect(status, link.OriginalUrl)
}

func (h *LinkHandler) GetLinkVisits(c *gin.Context) {
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

    total, err := h.queries.CountLinkVisits(c)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    visits, err := h.queries.GetLinkVisits(c, db.GetLinkVisitsParams{
        Limit:  int32(limit),
        Offset: int32(offset),
    })
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.Header("Content-Range", fmt.Sprintf("link_visits %d-%d/%d", start, start+len(visits)-1, total))
    c.JSON(http.StatusOK, visits)
}

func (h *LinkHandler) GetLinkVisitsByLinkID(c *gin.Context) {
    linkID, err := strconv.ParseInt(c.Param("id"), 10, 32)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid link id"})
        return
    }

    rangeParam := c.Query("range")

    var start, end int
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

    total, err := h.queries.CountLinkVisitsByLinkID(c, int32(linkID))
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    visits, err := h.queries.GetLinkVisitsByLinkID(c, db.GetLinkVisitsByLinkIDParams{
        LinkID: int32(linkID),
        Limit:  int32(limit),
        Offset: int32(offset),
    })
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.Header("Content-Range", fmt.Sprintf("link_visits %d-%d/%d", start, start+len(visits)-1, total))
    c.JSON(http.StatusOK, visits)
}

func FormatValidationError(err error) ValidationErrors {
    result := ValidationErrors{
        Errors: make(map[string]string),
    }

    if validationErrors, ok := err.(validator.ValidationErrors); ok {
        for _, e := range validationErrors {
            field := e.Field()
            var jsonField string

            switch field {
            case "OriginalURL":
                jsonField = "original_url"
            case "ShortName":
                jsonField = "short_name"
            default:
                jsonField = strings.ToLower(field)
            }

            switch e.Tag() {
            case "required":
                result.Errors[jsonField] = jsonField + " is required"
            case "url":
                result.Errors[jsonField] = jsonField + " must be a valid URL"
            case "min":
                result.Errors[jsonField] = jsonField + " must be at least " + e.Param() + " characters"
            case "max":
                result.Errors[jsonField] = jsonField + " must be at most " + e.Param() + " characters"
            default:
                result.Errors[jsonField] = jsonField + " is invalid"
            }
        }
    }

    return result
}

func main() {
	dbConn, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
        if err != nil {
            log.Fatal("Failed to connect to database:", err)
        }
        defer func() {
            if err := dbConn.Close(); err != nil {
                log.Printf("Error closing database connection: %v", err)
            }
        }()

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

    router.GET("/r/:code", linkHandler.RedirectHandler)

    api.GET("/link_visits", linkHandler.GetLinkVisits)
    api.GET("/links/:id/visits", linkHandler.GetLinkVisitsByLinkID)

	port := os.Getenv("APP_PORT")
    if port == "" {
        port = os.Getenv("PORT")
        if port == "" {
            port = "8080"
        }
    }

	log.Printf("Server starting on port %s", port)
	if err := router.Run(":" + port); err != nil {
        log.Fatal("Failed to start server:", err)
    }
}