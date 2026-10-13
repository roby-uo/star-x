package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountHandlerTest_RejectsInvalidOutputLimitBeforeUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewAccountHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.POST("/api/v1/admin/accounts/:id/test", handler.Test)
	for _, body := range []string{
		`{"max_output_tokens":0}`,
		`{"max_output_tokens":-1}`,
		`{"max_output_tokens":65}`,
		`{"max_output_tokens":"8"}`,
		`{"max_output_tokens":8.5}`,
		`{"max_output_tokens":99999999999999999999999}`,
		`{"max_output_tokens":8,`,
	} {
		t.Run(body, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/5/test", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(rec, req)
			require.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

type accountConnectionTestRepo struct {
	service.AccountRepository
}

func (r *accountConnectionTestRepo) GetByID(_ context.Context, id int64) (*service.Account, error) {
	return &service.Account{ID: id, Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey}, nil
}

func TestAccountHandlerTest_AbsentOutputLimitPreservesLegacyPlatformTest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	accountTestSvc := service.NewAccountTestService(&accountConnectionTestRepo{}, nil, nil, nil, nil, nil, nil, nil)
	handler := NewAccountHandler(nil, nil, nil, nil, nil, nil, nil, nil, accountTestSvc, nil, nil, nil, nil, nil)
	router := gin.New()
	router.POST("/api/v1/admin/accounts/:id/test", handler.Test)
	for _, body := range []string{"", `{"model_id":"claude-sonnet-4-6","prompt":"test"}`} {
		t.Run(body, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/5/test", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code)
			require.Contains(t, rec.Body.String(), "No API key available")
			require.NotContains(t, rec.Body.String(), "Bounded account tests")
		})
	}
}
