package repository

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

func http2KeepAliveTestPoolSettings() poolSettings {
	return poolSettings{
		maxIdleConns:          10,
		maxIdleConnsPerHost:   5,
		maxConnsPerHost:       10,
		idleConnTimeout:       90 * time.Second,
		responseHeaderTimeout: time.Minute,
	}
}

// Codex/OpenAI 上游改走 HTTP/2 后，池化连接被代理/NAT 静默掐断会成为“死连接”：
// 两端都以为连接存活，请求落上去会挂到 TCP 重传超时（分钟级）才失败。Go 的
// http2.Transport 默认 ReadIdleTimeout=0（不发健康 PING），无法检测这种死连接。
// 必须显式启用主动 PING 探测，让死连接被提前剔除，而不是只靠 ResponseHeaderTimeout
// 事后兜底。
func TestEnableOpenAIHTTP2KeepAlive_EnablesPingHealthCheck(t *testing.T) {
	tr := &http.Transport{}

	h2 := enableOpenAIHTTP2KeepAlive(tr)
	require.NotNil(t, h2, "必须返回原生 HTTP2Config")

	require.Positive(t, h2.SendPingTimeout, "必须启用空闲 PING 探测以剔除死连接")
	require.Equal(t, openAIHTTP2ReadIdleTimeout, h2.SendPingTimeout)
	require.Equal(t, openAIHTTP2PingTimeout, h2.PingTimeout, "PING 无响应必须有超时判定")
	require.True(t, tr.Protocols.HTTP2(), "原生协议配置必须启用HTTP/2")
	require.True(t, tr.Protocols.HTTP1(), "未协商H2时仍允许HTTP/1")
}

// openai_h2 模式构建的 Transport 必须带上 H2 PING 健康探测，从源头剔除死连接。
func TestBuildUpstreamTransport_OpenAIH2_EnablesPingHealthCheck(t *testing.T) {
	tr, err := buildUpstreamTransport(http2KeepAliveTestPoolSettings(), nil, upstreamProtocolModeOpenAIH2)
	require.NoError(t, err)
	require.True(t, tr.ForceAttemptHTTP2, "openai_h2 必须启用 HTTP/2")
	require.True(t, tr.Protocols.HTTP2())
	require.Equal(t, openAIHTTP2ReadIdleTimeout, tr.HTTP2.SendPingTimeout)
}

// 非 H2 模式（default/h1）不应因本次改动被误配置：default 走 Go 自动 H2（惰性配置，
// 构建时 TLSNextProto 仍为空），h1 模式显式禁用 H2。避免波及 Claude/Gemini 热路径。
func TestBuildUpstreamTransport_NonOpenAIH2_NotEagerlyConfigured(t *testing.T) {
	tr, err := buildUpstreamTransport(http2KeepAliveTestPoolSettings(), nil, upstreamProtocolModeDefault)
	require.NoError(t, err)
	require.Nil(t, tr.TLSNextProto["h2"], "default 模式不应在构建期主动配置 http2 keepalive")
}

// 死连接在经 HTTP 代理（CONNECT 隧道）时最高发，这是带 proxy 账号的真实生产路径：
// 显式 http2 配置须与 Transport.Proxy 同时正确生效，不能相互干扰。
func TestBuildUpstreamTransport_OpenAIH2_WithHTTPProxy_EnablesKeepAlive(t *testing.T) {
	proxyURL, err := url.Parse("http://127.0.0.1:8080")
	require.NoError(t, err)

	tr, err := buildUpstreamTransport(http2KeepAliveTestPoolSettings(), proxyURL, upstreamProtocolModeOpenAIH2)
	require.NoError(t, err)
	require.True(t, tr.ForceAttemptHTTP2)
	require.True(t, tr.Protocols.HTTP2())
	require.Equal(t, openAIHTTP2ReadIdleTimeout, tr.HTTP2.SendPingTimeout)
	require.NotNil(t, tr.Proxy, "HTTP 代理仍须通过 Transport.Proxy 生效")
}

func TestBuildUpstreamTransportNativeHTTP2NegotiationAndHTTP1Fallback(t *testing.T) {
	for _, enableH2 := range []bool{true, false} {
		t.Run(strconv.FormatBool(enableH2), func(t *testing.T) {
			upstream := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, strconv.Itoa(r.ProtoMajor)) }))
			upstream.EnableHTTP2 = enableH2
			upstream.StartTLS()
			defer upstream.Close()
			tr, err := buildUpstreamTransport(http2KeepAliveTestPoolSettings(), nil, upstreamProtocolModeOpenAIH2)
			require.NoError(t, err)
			roots := x509.NewCertPool()
			roots.AddCert(upstream.Certificate())
			tr.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
			defer tr.CloseIdleConnections()
			client := &http.Client{Transport: tr, Timeout: 3 * time.Second}
			response, err := client.Get(upstream.URL)
			require.NoError(t, err)
			defer func() { _ = response.Body.Close() }()
			body, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			want := 1
			if enableH2 {
				want = 2
			}
			require.Equal(t, want, response.ProtoMajor)
			require.Equal(t, strconv.Itoa(want), string(body))
		})
	}
}

func TestNativeHTTP2KeepAliveActuallySendsIdlePing(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()
	ping := make(chan struct{}, 1)
	peerDone := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			peerDone <- err
			return
		}
		defer func() { _ = conn.Close() }()
		preface := make([]byte, len(http2.ClientPreface))
		if _, err = io.ReadFull(conn, preface); err != nil {
			peerDone <- err
			return
		}
		frames := http2.NewFramer(conn, conn)
		if err = frames.WriteSettings(); err != nil {
			peerDone <- err
			return
		}
		for {
			frame, readErr := frames.ReadFrame()
			if readErr != nil {
				peerDone <- readErr
				return
			}
			switch frame := frame.(type) {
			case *http2.SettingsFrame:
				if !frame.IsAck() {
					err = frames.WriteSettingsAck()
				}
			case *http2.HeadersFrame:
				var block bytes.Buffer
				encoder := hpack.NewEncoder(&block)
				if err = encoder.WriteField(hpack.HeaderField{Name: ":status", Value: "200"}); err == nil {
					err = frames.WriteHeaders(http2.HeadersFrameParam{StreamID: frame.StreamID, EndHeaders: true, EndStream: true, BlockFragment: block.Bytes()})
				}
			case *http2.PingFrame:
				if !frame.IsAck() {
					select {
					case ping <- struct{}{}:
					default:
					}
					err = frames.WritePing(true, frame.Data)
				}
			}
			if err != nil {
				peerDone <- err
				return
			}
		}
	}()
	tr := &http.Transport{}
	h2 := enableOpenAIHTTP2KeepAlive(tr)
	h2.SendPingTimeout = 20 * time.Millisecond
	h2.PingTimeout = 200 * time.Millisecond
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	tr.Protocols = protocols
	defer tr.CloseIdleConnections()
	response, err := (&http.Client{Transport: tr, Timeout: 3 * time.Second}).Get("http://" + listener.Addr().String())
	require.NoError(t, err)
	require.Equal(t, 2, response.ProtoMajor)
	_, err = io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	select {
	case <-ping:
	case err := <-peerDone:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("native HTTP2Config did not emit a health PING on the idle connection")
	}
}
