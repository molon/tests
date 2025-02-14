package http_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type panicHandler struct{}

func (h *panicHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("name") == "panic" {
		time.Sleep(100 * time.Millisecond) // 给其他请求一些时间先执行
		panic("test panic")
	}
	time.Sleep(50 * time.Millisecond) // 模拟一些处理时间
	fmt.Fprintf(w, "Hello %s", r.URL.Query().Get("name"))
}

func TestHTTPServerPanic(t *testing.T) {
	t.Run("WithRecovery", func(t *testing.T) {
		t.Log("Starting test with recovery...")

		// 创建带恢复机制的处理器
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if r := recover(); r != nil {
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprintf(w, "panic occurred: %v", r)
				}
			}()
			(&panicHandler{}).ServeHTTP(w, r)
		})

		// 创建测试服务器
		server := httptest.NewServer(handler)
		defer server.Close()

		// 先发送一个正常请求，确保服务正常
		resp, err := http.Get(server.URL + "?name=test")
		assert.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		assert.NoError(t, err)
		assert.Equal(t, "Hello test", string(body))

		// 发送会导致 panic 的请求
		resp, err = http.Get(server.URL + "?name=panic")
		assert.NoError(t, err)
		body, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		assert.NoError(t, err)
		assert.Contains(t, string(body), "panic occurred")
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

		// 再次发送请求，验证服务器是否仍在运行
		resp, err = http.Get(server.URL + "?name=after_panic")
		assert.NoError(t, err)
		body, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		assert.NoError(t, err)
		assert.Equal(t, "Hello after_panic", string(body))
	})

	t.Run("WithoutRecovery", func(t *testing.T) {
		t.Log("Starting test with recovery...")

		// 创建测试服务器
		server := httptest.NewServer(&panicHandler{})
		defer server.Close()

		// 先发送一个正常请求，确保服务正常
		resp, err := http.Get(server.URL + "?name=test")
		assert.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		assert.NoError(t, err)
		assert.Equal(t, "Hello test", string(body))

		// 发送会导致 panic 的请求
		resp, err = http.Get(server.URL + "?name=panic")
		assert.ErrorContains(t, err, "EOF")
		assert.Nil(t, resp)

		// 再次发送请求，发现服务器仍在运行
		resp, err = http.Get(server.URL + "?name=after_panic")
		assert.NoError(t, err)
		body, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		assert.NoError(t, err)
		assert.Equal(t, "Hello after_panic", string(body))

		// !!! 结论是，在没有 recovery中间件 的情况下，进程也不会挂掉，http server 内部处理了 recover 但是依然推荐自行再处理一次 !!!
	})
}
