package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/davidmuller5273-boop/safe/internal/service"
)

func TestLotteryBroadcastRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := Router(nil, service.Auth{Secret: []byte("x")})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/lottery-broadcast/ui", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "开奖播报") {
		t.Fatalf("ui status=%d", w.Code)
	}
	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/admin/lottery-broadcast", nil),
		httptest.NewRequest(http.MethodPut, "/admin/lottery-broadcast/switches", strings.NewReader(`{}`)),
		httptest.NewRequest(http.MethodDelete, "/admin/lottery-broadcast/subscriptions", strings.NewReader(`{}`)),
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without token = %d, want 401", req.Method, req.URL.Path, w.Code)
		}
	}
}
