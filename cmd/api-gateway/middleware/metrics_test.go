package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestHTTPMetricsMiddleware_RecordsRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := prometheus.NewRegistry()
	metrics := NewHTTPMetrics(registry)

	router := gin.New()
	router.Use(metrics.Middleware())
	router.GET("/ok", func(c *gin.Context) {
		c.Status(http.StatusCreated)
	})

	req, err := http.NewRequest(http.MethodGet, "/ok", nil)
	assert.NoError(t, err)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusCreated, recorder.Code)

	requestTotal := testutil.ToFloat64(metrics.requestsTotal.WithLabelValues("GET", "/ok", "201"))
	assert.Equal(t, float64(1), requestTotal)
}
