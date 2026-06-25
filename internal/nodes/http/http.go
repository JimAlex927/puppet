package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	nethttp "net/http"
	"strings"
	"time"

	"puppet/internal/node"
)

type Executor struct {
	client *nethttp.Client
}

func New() *Executor {
	return &Executor{client: &nethttp.Client{Timeout: 30 * time.Second}}
}

func (e *Executor) Type() string {
	return "http"
}

func (e *Executor) Metadata() node.NodeMetadata {
	return node.NodeMetadata{
		Type:        e.Type(),
		Name:        "HTTP Request",
		Category:    "network",
		Description: "发送 HTTP 请求并校验 2xx 状态码",
		SupportedOS: []string{"linux", "darwin", "windows"},
		Fields: []node.NodeField{
			{Name: "method", Label: "Method", Type: "select", Required: true, Default: "GET", Options: []string{"GET", "POST", "PUT", "DELETE"}},
			{Name: "url", Label: "URL", Type: "input", Required: true},
			{Name: "headers", Label: "Headers JSON", Type: "textarea", Required: false, Default: "{}"},
			{Name: "body", Label: "Body", Type: "textarea", Required: false},
		},
	}
}

func (e *Executor) Validate(params map[string]any) error {
	if strings.TrimSpace(stringFrom(params["url"])) == "" {
		return fmt.Errorf("url is required")
	}
	return nil
}

func (e *Executor) Execute(ctx *node.NodeContext, params map[string]any) (*node.NodeResult, error) {
	method := strings.ToUpper(stringFrom(params["method"]))
	if method == "" {
		method = "GET"
	}
	url := stringFrom(params["url"])
	body := stringFrom(params["body"])

	req, err := nethttp.NewRequestWithContext(ctx.Context, method, url, bytes.NewBufferString(body))
	if err != nil {
		return nil, err
	}
	for key, value := range headersFrom(params["headers"]) {
		req.Header.Set(key, value)
	}
	if body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	ctx.Log("stdout", fmt.Sprintf("%s %s\n", method, url))
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	content, _ := io.ReadAll(io.LimitReader(resp.Body, 1000))
	ctx.Log("stdout", fmt.Sprintf("response status: %d\n", resp.StatusCode))
	if len(content) > 0 {
		ctx.Log("stdout", string(content)+"\n")
	}

	output := map[string]any{"statusCode": resp.StatusCode, "body": string(content)}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &node.NodeResult{Output: output}, fmt.Errorf("http status %d", resp.StatusCode)
	}
	return &node.NodeResult{Output: output}, nil
}

func headersFrom(value any) map[string]string {
	headers := map[string]string{}
	switch typed := value.(type) {
	case map[string]any:
		for key, raw := range typed {
			headers[key] = stringFrom(raw)
		}
	case map[string]string:
		return typed
	case string:
		var decoded map[string]string
		if err := json.Unmarshal([]byte(typed), &decoded); err == nil {
			return decoded
		}
		var loose map[string]any
		if err := json.Unmarshal([]byte(typed), &loose); err == nil {
			for key, raw := range loose {
				headers[key] = stringFrom(raw)
			}
		}
	}
	return headers
}

func stringFrom(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}
