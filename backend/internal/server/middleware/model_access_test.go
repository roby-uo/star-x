//go:build unit

package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestModelAccessGuardEnforcesHTTPAndWebSocket(t *testing.T) {
	for _, tc := range []struct {
		name, method, body, upgrade string
		allowed                     bool
	}{
		{"allowed", http.MethodPost, `{"model":"MiniMax-H3","duration":5,"resolution":"768P"}`, "", true},
		{"blocked-resolution", http.MethodPost, `{"model":"MiniMax-H3","duration":5,"resolution":"2K"}`, "", false},
		{"blocked-model", http.MethodPost, `{"model":"other"}`, "", false},
		{"websocket-cannot-bypass-whitelist", http.MethodGet, "", "websocket", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			tasks := service.NewMediaTaskService(db, nil, nil, &service.OpenAIGatewayService{})
			defer tasks.Stop()
			mock.ExpectQuery("SELECT rules FROM api_key_model_access").WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"rules"}).AddRow(`{"restrict_models":true,"models":["MiniMax-H3"],"video_resolutions":["768P"],"video_max_duration":5}`))
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(tc.method, "/v1/responses", strings.NewReader(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Request.Header.Set("Upgrade", tc.upgrade)
			require.Equal(t, tc.allowed, ModelAccessCheck(tasks)(c, &service.APIKey{ID: 1}))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
