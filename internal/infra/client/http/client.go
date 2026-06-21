package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/moul/http2curl"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	CTJSON = "json"
	CTForm = "form"
)

var DefaultMaskedFields = []string{
	"private_key",
	"Authorization",
	"auth_token",
	"access_token",
	"api_key",
}

// Client 对 *http.Client 的便捷封装
type Client struct {
	HTTP         *http.Client
	MaskedFields []string
}

// Response 把状态码、头、Body、已脱敏 curl 一并返回，便于在 service 里日志打印
type Response struct {
	Status int
	Body   []byte
	Header http.Header
	Curl   string // 已脱敏
}

// NewClient 基于 Option 生成带可观测能力的 HTTP 客户端
func NewClient(opt Option) *Client {
	if opt.Timeout == 0 {
		opt.Timeout = 15 * time.Second
	}
	return &Client{
		HTTP:         New(opt),
		MaskedFields: DefaultMaskedFields,
	}
}

// Do 统一请求（GET/POST/PUT/DELETE；POST/PUT 支持 json/form）
func (c *Client) Do(
	ctx context.Context,
	method, rawURL string,
	query map[string]string,
	body any,
	headers map[string]string,
	ctype string, // CTJSON | CTForm | ""
) (*Response, error) {
	if c.HTTP == nil {
		return nil, errors.New("nil http client")
	}
	// URL 与 query
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if len(query) > 0 {
		q := u.Query()
		for k, v := range query {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
	}

	// Body & Content-Type
	var rdr io.Reader
	if (method == http.MethodPost || method == http.MethodPut) && body != nil {
		switch strings.ToLower(ctype) {
		case CTJSON:
			bts, err := json.Marshal(body)
			if err != nil {
				return nil, err
			}
			rdr = bytes.NewReader(bts)
		case CTForm:
			values := url.Values{}
			// 仅支持 map[string]string 的简单表单
			if m, ok := body.(map[string]string); ok {
				for k, v := range m {
					values.Set(k, v)
				}
			} else {
				return nil, errors.New("form body must be map[string]string")
			}
			rdr = strings.NewReader(values.Encode())
		default:
			// 不设置：由调用方通过 headers 自己指定
		}
	}

	//  request
	req, err := http.NewRequestWithContext(ctx, method, u.String(), rdr)
	if err != nil {
		return nil, err
	}
	if method == http.MethodPost || method == http.MethodPut {
		switch strings.ToLower(ctype) {
		case CTJSON:
			req.Header.Set("Content-Type", "application/json")
		case CTForm:
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// curl（先生成再发送，方便失败时也能拿到）
	curlCmd, _ := http2curl.GetCurlCommand(req)
	maskedCurl := maskSensitive(curlCmd.String(), c.MaskedFields)

	// 发送
	resp, err := c.HTTP.Do(req)
	if err != nil {
		annotateHTTPFailureSpan(ctx, method, rawURL, 0, maskedCurl, nil, err)
		return &Response{Status: 0, Body: nil, Header: http.Header{}, Curl: maskedCurl}, err
	}
	defer resp.Body.Close()

	bts, _ := io.ReadAll(resp.Body)
	res := &Response{
		Status: resp.StatusCode,
		Body:   bts,
		Header: resp.Header.Clone(),
		Curl:   maskedCurl,
	}
	// 2xx 认为成功，其它交给调用方判断/处理
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return res, fmt.Errorf("http %d: %s", resp.StatusCode, string(bts))
	}
	return res, nil
}

// UploadFile 上传文件
type UploadFile struct {
	FieldName string // 表单字段名
	FileName  string // 文件名
	Content   []byte // 文件内容
}

// DoMultipart multipart/form-data（多文件 + 普通字段）
func (c *Client) DoMultipart(
	ctx context.Context,
	method, rawURL string,
	fields map[string]string,
	files []UploadFile,
	headers map[string]string,
) (*Response, error) {
	if c.HTTP == nil {
		return nil, errors.New("nil http client")
	}

	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)

	for k, v := range fields {
		if err := writer.WriteField(k, v); err != nil {
			return nil, err
		}
	}
	for _, f := range files {
		part, err := writer.CreateFormFile(f.FieldName, f.FileName)
		if err != nil {
			return nil, err
		}
		if _, err := part.Write(f.Content); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, payload)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	curlCmd, _ := http2curl.GetCurlCommand(req)
	maskedCurl := maskSensitive(curlCmd.String(), c.MaskedFields)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		annotateHTTPFailureSpan(ctx, method, rawURL, 0, maskedCurl, nil, err)
		return &Response{Status: 0, Body: nil, Header: http.Header{}, Curl: maskedCurl}, err
	}
	defer resp.Body.Close()

	bts, _ := io.ReadAll(resp.Body)
	res := &Response{
		Status: resp.StatusCode,
		Body:   bts,
		Header: resp.Header.Clone(),
		Curl:   maskedCurl,
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return res, fmt.Errorf("http %d: %s", resp.StatusCode, string(bts))
	}
	return res, nil
}

// ==== 工具：敏感字段脱敏 ====
func maskSensitive(input string, fields []string) string {
	masked := input
	for _, f := range fields {
		// JSON: "private_key":"xxxxx"
		jsonPattern := regexp.MustCompile(fmt.Sprintf(`"%s"\s*:\s*"[^"]*"`, regexp.QuoteMeta(f)))
		masked = jsonPattern.ReplaceAllString(masked, fmt.Sprintf(`"%s":"[敏感参数]"`, f))

		// form/curl: private_key=xxxxx
		formPattern := regexp.MustCompile(fmt.Sprintf(`%s=[^&\s]*`, regexp.QuoteMeta(f)))
		masked = formPattern.ReplaceAllString(masked, fmt.Sprintf(`%s=[敏感参数]`, f))

		// header: Authorization: xxxx
		headerPattern := regexp.MustCompile(fmt.Sprintf(`(?i)%s:\s*[^\r\n]+`, regexp.QuoteMeta(f)))
		masked = headerPattern.ReplaceAllString(masked, fmt.Sprintf(`%s: [敏感参数]`, f))
	}
	return masked
}

// 截断，避免把超大响应体塞进 span
func clip(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}

// 失败时把关键信息打到当前 span（通常是上游 HTTP handler 或业务 span）
func annotateHTTPFailureSpan(
	ctx context.Context,
	method, urlStr string,
	status int,
	curlMasked string,
	respBody []byte,
	err error,
) {
	span := trace.SpanFromContext(ctx)
	if span == nil || !span.SpanContext().IsValid() {
		return
	}

	attrs := []attribute.KeyValue{
		semconv.HTTPMethodKey.String(method),
		semconv.URLFull(urlStr), // 等价 semconv.HTTPScheme/HTTPHost/Target 组合
		attribute.Int("http.status_code", status),
		attribute.String("http.curl", clip(curlMasked, 2048)),
	}
	if len(respBody) > 0 {
		attrs = append(attrs, attribute.String("http.response.body", clip(string(respBody), 2048)))
	}

	// 记录事件
	span.AddEvent("http.client.failure", trace.WithAttributes(attrs...))

	// 标记错误状态 + 记录错误
	var msg string
	if err != nil {
		msg = err.Error()
		span.RecordError(err)
	} else if status >= 400 {
		msg = fmt.Sprintf("http %d", status)
	}
	if msg != "" {
		span.SetStatus(codes.Error, msg)
	}
}
