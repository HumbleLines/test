package spotsigner

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"mime"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// 默认常量。
const (
	DefaultAlgorithm  = "HmacSHA256"
	DefaultRecvWindow = "5000"
)

// BuildHeadersInput 生成签名请求头的输入参数。
type BuildHeadersInput struct {
	// HTTP 方法，例如 GET/POST/DELETE/PUT。
	Method string

	// 请求路径，只传 path，例如：/v4/order
	// 不要传完整域名 URL。
	Path string

	// 原始 query 参数。
	// 如果没有 query，可以传 nil。
	Query url.Values

	// Content-Type，例如：
	// application/json
	// application/x-www-form-urlencoded
	ContentType string

	// 原始请求体字节。
	// JSON/raw：传原始 body
	// urlencoded：传原始表单字符串
	Body []byte

	// validate-appkey
	AppKey string

	// HMAC secret
	SecretKey string

	// validate-recvwindow，默认 5000
	RecvWindow string

	// validate-algorithms，默认 HmacSHA256
	Algorithm string

	// 时间函数，方便测试；不传默认 time.Now
	NowFunc func() time.Time
}

// BuildHeadersResult 生成结果。
type BuildHeadersResult struct {
	// 最终 headers，可直接设置到请求头。
	Headers map[string]string

	// 以下字段只是调试或单测时可能有用；
	// 你不用可以不关心。
	Timestamp string
	X         string
	Y         string
	Original  string
	Signature string
}

// BuildHeaders 生成 XT / Siristars 风格签名请求头。
// 返回值中的 Headers 即可直接用于 HTTP 请求头。
func BuildHeaders(in BuildHeadersInput) (*BuildHeadersResult, error) {
	if strings.TrimSpace(in.AppKey) == "" {
		return nil, errors.New("spotsigner: empty app key")
	}
	if strings.TrimSpace(in.SecretKey) == "" {
		return nil, errors.New("spotsigner: empty secret key")
	}

	method := strings.ToUpper(strings.TrimSpace(in.Method))
	if method == "" {
		method = "GET"
	}

	path := strings.TrimSpace(in.Path)
	if path == "" {
		return nil, errors.New("spotsigner: empty path")
	}

	if strings.TrimSpace(in.RecvWindow) == "" {
		in.RecvWindow = DefaultRecvWindow
	}
	if strings.TrimSpace(in.Algorithm) == "" {
		in.Algorithm = DefaultAlgorithm
	}
	if in.NowFunc == nil {
		in.NowFunc = time.Now
	}

	// 1) timestamp
	ts := strconvUnixMilli(in.NowFunc())

	// 2) queryStr
	queryStr := buildSortedQueryString(in.Query)

	// 3) bodyStr
	bodyStr, err := buildBodyString(in.ContentType, in.Body)
	if err != nil {
		return nil, err
	}

	// 4) Y = #METHOD#PATH[#QUERY][#BODY]
	y := "#" + method + "#" + path
	if queryStr != "" {
		y += "#" + queryStr
	}
	if bodyStr != "" {
		y += "#" + bodyStr
	}

	// 5) X：header key 按字母序
	headersForSign := map[string]string{
		"validate-algorithms": in.Algorithm,
		"validate-appkey":     in.AppKey,
		"validate-recvwindow": in.RecvWindow,
		"validate-timestamp":  ts,
	}

	keys := make([]string, 0, len(headersForSign))
	for k := range headersForSign {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var xb strings.Builder
	for i, k := range keys {
		if i > 0 {
			xb.WriteByte('&')
		}
		xb.WriteString(k)
		xb.WriteByte('=')
		xb.WriteString(headersForSign[k])
	}
	x := xb.String()

	// 6) original = X + Y
	original := x + y

	// 7) signature
	signature := hmacSHA256Hex(in.SecretKey, original)

	// 8) 最终 headers
	headers := map[string]string{
		"validate-algorithms": in.Algorithm,
		"validate-appkey":     in.AppKey,
		"validate-recvwindow": in.RecvWindow,
		"validate-timestamp":  ts,
		"validate-signature":  signature,
	}

	return &BuildHeadersResult{
		Headers:   headers,
		Timestamp: ts,
		X:         x,
		Y:         y,
		Original:  original,
		Signature: signature,
	}, nil
}

// buildSortedQueryString 按 key、再按 value 排序 query。
func buildSortedQueryString(values url.Values) string {
	if len(values) == 0 {
		return ""
	}

	type pair struct {
		Key   string
		Value string
	}

	pairs := make([]pair, 0)
	for k, vals := range values {
		if len(vals) == 0 {
			pairs = append(pairs, pair{Key: k, Value: ""})
			continue
		}
		for _, v := range vals {
			pairs = append(pairs, pair{Key: k, Value: v})
		}
	}

	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].Key != pairs[j].Key {
			return pairs[i].Key < pairs[j].Key
		}
		return pairs[i].Value < pairs[j].Value
	})

	var b strings.Builder
	for i, p := range pairs {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(p.Key)
		b.WriteByte('=')
		b.WriteString(p.Value)
	}

	return b.String()
}

// buildBodyString 构造参与签名的 body 字符串。
func buildBodyString(contentType string, body []byte) (string, error) {
	if len(body) == 0 {
		return "", nil
	}

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = strings.TrimSpace(strings.ToLower(contentType))
	}
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))

	switch {
	case strings.HasPrefix(mediaType, "multipart/form-data"):
		return "", errors.New("spotsigner: form-data / multipart is not supported")

	case mediaType == "application/x-www-form-urlencoded":
		formValues, err := url.ParseQuery(string(body))
		if err != nil {
			return "", err
		}
		return buildSortedQueryString(formValues), nil

	default:
		// raw/json：按原始字符串参与签名，只去掉末尾连续换行
		return strings.TrimRight(string(body), "\r\n"), nil
	}
}

// hmacSHA256Hex 计算 HMAC SHA256 十六进制小写串。
func hmacSHA256Hex(secret, original string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(original))
	return hex.EncodeToString(mac.Sum(nil))
}

// strconvUnixMilli 将时间转毫秒字符串。
func strconvUnixMilli(t time.Time) string {
	return strconv.FormatInt(t.UnixMilli(), 10)
}
