package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBillingManagementRejectsCredentialsAndTrailingJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, body := range []string{`{"version":0,"api_key":"must-never-be-stored"}`, `{"version":0} {"version":1}`, `{"cost_profiles":[{"id":"a","credentials":{"token":"no"}}]}`} {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
		handler := NewBillingManagementHandler(nil)
		handler.UpdateConfig(ctx)
		require.Equal(t, http.StatusBadRequest, recorder.Code)
		require.NotContains(t, recorder.Body.String(), "must-never-be-stored")
	}
}
