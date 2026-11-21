package selfupdate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Autumn-27/artex/internal/httpidentity"
)

// Repo 是发布源。写死而不是做成配置项：更新源可配等于给任何能改配置的人一条
// 远程代码执行通道，对一个渗透测试平台来说这个口子开不得。
const Repo = "yzbenyu-wq/ARTEX"

// latestURL 是 GitHub 的"最新正式版"接口。它会自动跳过 prerelease 和 draft。
const latestURL = "https://api.github.com/repos/" + Repo + "/releases/latest"

// allowedHosts 限定升级链路能访问的域名。配合下面的 checkRedirect，
// 任何一跳被重定向到名单外的主机都会直接失败——这是防止 DNS 污染 / 中间人
// 把二进制换掉的第一道闸门，第二道是 SHA256SUMS 比对。
var allowedHosts = map[string]bool{
	"api.github.com":                       true,
	"github.com":                           true,
	"objects.githubusercontent.com":        true, // release 资产实际落地的对象存储
	"release-assets.githubusercontent.com": true,
	"raw.githubusercontent.com":            true,
}

// Release 是 GitHub Release 里我们关心的字段。
type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
	Assets      []Asset   `json:"assets"`
}

// Asset 是 Release 上挂的一个文件。
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

// ParseProxy validates an optional update proxy without including credentials in errors.
func ParseProxy(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("代理地址格式无效")
	}
	switch u.Scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return nil, fmt.Errorf("代理协议须为 http、https、socks5 或 socks5h")
	}
	if u.Hostname() == "" || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("代理地址须包含主机，且不能包含路径、查询或片段")
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("代理端口须在 1–65535 之间")
		}
	} else if strings.HasSuffix(u.Host, ":") {
		return nil, fmt.Errorf("代理端口不能为空")
	}
	return u, nil
}

// NewClient validates the proxy before opening any connection. Empty means direct,
// regardless of HTTP_PROXY / HTTPS_PROXY in the process environment.
func NewClient(proxy string) (*http.Client, error) {
	pu, err := ParseProxy(proxy)
	if err != nil {
		return nil, err
	}
	tr := &http.Transport{
		ForceAttemptHTTP2:   true,
		TLSHandshakeTimeout: 15 * time.Second,
	}
	if pu != nil {
		tr.Proxy = http.ProxyURL(pu)
	}

	return &http.Client{
		Transport: tr,
		Timeout:   30 * time.Minute, // 下载整包，不能按请求级超时卡死
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("重定向次数过多")
			}
			return checkURL(req.URL)
		},
	}, nil
}

// checkURL 强制 https + 域名白名单。
func checkURL(u *url.URL) error {
	if u.Scheme != "https" {
		return fmt.Errorf("拒绝非 HTTPS 地址: %s", u.Scheme+"://"+u.Host)
	}
	if !allowedHosts[strings.ToLower(u.Hostname())] {
		return fmt.Errorf("拒绝非 GitHub 域名: %s", u.Hostname())
	}
	return nil
}

// FetchError 是查询发布源失败时带分类的错误，供前端按原因给出可操作提示。
// 直接比较错误文本既脆弱又会把局部改动扩散到 UI，故用类型携带分类。
type FetchError struct {
	Kind string
	Err  error
}

func (e *FetchError) Error() string { return e.Err.Error() }
func (e *FetchError) Unwrap() error { return e.Err }

// 分类取值；前端 settings.updateErrorHint* 文案与之一一对应。
const (
	FetchKindNetwork   = "network"          // 连不上 GitHub（离线/内网/DNS/代理不通）
	FetchKindRateLimit = "rate_limited"     // GitHub 未认证接口限流
	FetchKindNotFound  = "not_found"        // 仓库不可访问或没有正式发布版本
	FetchKindSecurity  = "security"         // 地址未通过 https/域名白名单校验
	FetchKindHTTP      = "http"             // 其他非 200 响应
	FetchKindInvalid   = "invalid_response" // 响应无法解析
)

// KindOf 返回 FetchLatest 失败的分类；成功返回空串，非分类错误归为 "other"。
func KindOf(err error) string {
	if err == nil {
		return ""
	}
	var fe *FetchError
	if errors.As(err, &fe) {
		return fe.Kind
	}
	return "other"
}

// FetchLatest 查询最新正式版。
func FetchLatest(ctx context.Context, c *http.Client) (*Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestURL, nil)
	if err != nil {
		return nil, &FetchError{Kind: FetchKindInvalid, Err: err}
	}
	if err := checkURL(req.URL); err != nil {
		return nil, &FetchError{Kind: FetchKindSecurity, Err: err}
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	httpidentity.Apply(req)

	resp, err := c.Do(req)
	if err != nil {
		return nil, &FetchError{
			Kind: FetchKindNetwork,
			Err:  fmt.Errorf("访问 GitHub 失败（可在版本与更新中配置更新代理）: %w", err),
		}
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusForbidden, resp.StatusCode == http.StatusTooManyRequests:
		// 未认证的 GitHub API 是每 IP 每小时 60 次，共用出口 IP 时很容易撞上。
		return nil, &FetchError{Kind: FetchKindRateLimit, Err: fmt.Errorf("GitHub 接口限流（每小时 60 次），请稍后再试")}
	case resp.StatusCode == http.StatusNotFound:
		return nil, &FetchError{Kind: FetchKindNotFound, Err: fmt.Errorf("仓库 %s 不可访问或尚未发布正式版本", Repo)}
	case resp.StatusCode != http.StatusOK:
		return nil, &FetchError{Kind: FetchKindHTTP, Err: fmt.Errorf("GitHub 返回 %d", resp.StatusCode)}
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, &FetchError{Kind: FetchKindInvalid, Err: fmt.Errorf("解析 Release 失败: %w", err)}
	}
	if strings.TrimSpace(rel.TagName) == "" {
		return nil, &FetchError{Kind: FetchKindInvalid, Err: fmt.Errorf("Release 缺少 tag")}
	}
	return &rel, nil
}

// AssetName 返回当前平台对应的发布包名，与 build.sh 的 package_binary 保持一致：
// artex-<版本>-<os>-<arch>.zip（版本号不带 v 前缀）。
func AssetName(tag, goos, goarch string) string {
	return fmt.Sprintf("artex-%s-%s-%s.zip", strings.TrimPrefix(tag, "v"), goos, goarch)
}

// FindAsset 在 Release 里按名字找资产。
func (r *Release) FindAsset(name string) (Asset, bool) {
	for _, a := range r.Assets {
		if strings.EqualFold(a.Name, name) {
			return a, true
		}
	}
	return Asset{}, false
}
