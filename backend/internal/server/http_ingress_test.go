//go:build unit

package server

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
)

func ingressTestConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Host:               "127.0.0.1",
			ReadHeaderTimeout:  1,
			IdleTimeout:        5,
			MaxHeaderBytes:     8 * 1024,
			MaxRequestBodySize: 1024,
		},
		Gateway: config.GatewayConfig{MaxBodySize: 1024},
	}
}

func TestProvideHTTPServerAppliesIngressLimits(t *testing.T) {
	srv := ProvideHTTPServer(ingressTestConfig(), gin.New())
	require.Equal(t, 8*1024, srv.MaxHeaderBytes)
	require.Equal(t, time.Second, srv.ReadHeaderTimeout)
	require.Equal(t, 5*time.Second, srv.IdleTimeout)
}

func TestProvideHTTPServerEnablesBoundedH2C(t *testing.T) {
	cfg := ingressTestConfig()
	cfg.Server.H2C = config.H2CConfig{
		Enabled:                      true,
		MaxConcurrentStreams:         25,
		IdleTimeout:                  30,
		MaxReadFrameSize:             64 * 1024,
		MaxUploadBufferPerConnection: 1024 * 1024,
		MaxUploadBufferPerStream:     256 * 1024,
	}
	srv := ProvideHTTPServer(cfg, gin.New())
	require.NotNil(t, srv.Protocols)
	require.True(t, srv.Protocols.UnencryptedHTTP2())
	require.True(t, srv.Protocols.HTTP1())
}

func TestHTTPServerH2CWireSettingsAndIndependentIdleTimeout(t *testing.T) {
	router := gin.New()
	router.GET("/", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	cfg := ingressTestConfig()
	cfg.Server.IdleTimeout = 5
	cfg.Server.H2C = config.H2CConfig{Enabled: true, MaxConcurrentStreams: 25, IdleTimeout: 1, MaxReadFrameSize: 64 * 1024, MaxUploadBufferPerConnection: 1024 * 1024, MaxUploadBufferPerStream: 256 * 1024}
	addr, stop := serveIngressTestServer(t, ProvideHTTPServer(cfg, router))
	defer stop()
	h1, err := net.DialTimeout("tcp", addr, time.Second)
	require.NoError(t, err)
	defer func() { _ = h1.Close() }()
	reader := bufio.NewReader(h1)
	http1Request := func() {
		require.NoError(t, h1.SetDeadline(time.Now().Add(3*time.Second)))
		_, err := io.WriteString(h1, "GET / HTTP/1.1\r\nHost: local\r\n\r\n")
		require.NoError(t, err)
		response, err := http.ReadResponse(reader, nil)
		require.NoError(t, err)
		_, err = io.ReadAll(response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusOK, response.StatusCode)
	}
	http1Request()
	h2, err := net.DialTimeout("tcp", addr, time.Second)
	require.NoError(t, err)
	defer func() { _ = h2.Close() }()
	require.NoError(t, h2.SetDeadline(time.Now().Add(2500*time.Millisecond)))
	_, err = io.WriteString(h2, http2.ClientPreface)
	require.NoError(t, err)
	frames := http2.NewFramer(h2, h2)
	require.NoError(t, frames.WriteSettings())
	sawSettings, sawGoAway := false, false
	for !sawGoAway {
		frame, err := frames.ReadFrame()
		require.NoError(t, err, "H2C must send GOAWAY at its own one-second idle limit")
		switch frame := frame.(type) {
		case *http2.SettingsFrame:
			if frame.IsAck() {
				continue
			}
			values := map[http2.SettingID]uint32{}
			require.NoError(t, frame.ForeachSetting(func(setting http2.Setting) error { values[setting.ID] = setting.Val; return nil }))
			require.Equal(t, uint32(25), values[http2.SettingMaxConcurrentStreams])
			require.Equal(t, uint32(64*1024), values[http2.SettingMaxFrameSize])
			require.Equal(t, uint32(256*1024), values[http2.SettingInitialWindowSize])
			sawSettings = true
			require.NoError(t, frames.WriteSettingsAck())
		case *http2.GoAwayFrame:
			require.Equal(t, http2.ErrCodeNo, frame.ErrCode)
			sawGoAway = true
		}
	}
	require.True(t, sawSettings)
	// Reuse the original H1 connection after H2 has timed out. A shared
	// Server.IdleTimeout override would incorrectly close this connection too.
	http1Request()
}

func TestConfigureTrustedProxies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name string
		cfg  config.ServerConfig
		want string
	}{
		{
			name: "configured proxy resolves forwarded client",
			cfg: config.ServerConfig{
				TrustedProxies:           []string{"9.9.9.9/32"},
				TrustedProxiesConfigured: true,
			},
			want: "1.2.3.4",
		},
		{
			name: "explicit empty list ignores forwarded client",
			cfg: config.ServerConfig{
				TrustedProxiesConfigured: true,
			},
			want: "9.9.9.9",
		},
		{
			name: "invalid proxy list fails closed",
			cfg: config.ServerConfig{
				TrustedProxies:           []string{"not-a-cidr"},
				TrustedProxiesConfigured: true,
			},
			want: "9.9.9.9",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			configureTrustedProxies(r, tc.cfg)
			r.GET("/t", func(c *gin.Context) { c.String(http.StatusOK, c.ClientIP()) })

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/t", nil)
			req.RemoteAddr = "9.9.9.9:12345"
			req.Header.Set("X-Forwarded-For", "1.2.3.4")
			r.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, tc.want, w.Body.String())
		})
	}
}

func TestHTTPServerRejectsOversizedHTTP1Header(t *testing.T) {
	r := gin.New()
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })
	srv := ProvideHTTPServer(ingressTestConfig(), r)
	addr, stop := serveIngressTestServer(t, srv)
	defer stop()

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	_, err = io.WriteString(conn, "GET / HTTP/1.1\r\nHost: test\r\nX-Fill: "+strings.Repeat("a", 32*1024)+"\r\n\r\n")
	require.NoError(t, err)
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusRequestHeaderFieldsTooLarge, resp.StatusCode)
}

func TestHTTPServerClosesSlowIncompleteHeader(t *testing.T) {
	r := gin.New()
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })
	srv := ProvideHTTPServer(ingressTestConfig(), r)
	addr, stop := serveIngressTestServer(t, srv)
	defer stop()

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	_, err = io.WriteString(conn, "GET / HTTP/1.1\r\nHost: test\r\nX-Slow:")
	require.NoError(t, err)
	time.Sleep(1200 * time.Millisecond)
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	_, err = bufio.NewReader(conn).ReadByte()
	require.Error(t, err)
}

func TestHTTPServerGlobalBodyLimit(t *testing.T) {
	r := gin.New()
	r.POST("/", func(c *gin.Context) {
		_, err := io.ReadAll(c.Request.Body)
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				c.Status(http.StatusRequestEntityTooLarge)
				return
			}
		}
		c.Status(http.StatusOK)
	})
	srv := ProvideHTTPServer(ingressTestConfig(), r)
	req, err := http.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", 1025)))
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}

func serveIngressTestServer(t *testing.T, srv *http.Server) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = srv.Serve(ln) }()
	return ln.Addr().String(), func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}
}
