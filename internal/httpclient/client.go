package httpclient

import (
	"fmt"
	"jsleaksscan/internal/config" // 导入配置包
	"net/http"
	"net/url"
	"time"
)

// CreateHTTPClient 根据提供的扫描选项创建和配置 HTTP 客户端
func CreateHTTPClient(opts config.ScanOptions) (*http.Client, error) {
	transport := &http.Transport{
		// 可以添加其他 Transport 配置，例如 TLS, KeepAlive 等
	}

	// 配置代理
	if opts.Proxy != "" {
		proxyURL, err := url.Parse(opts.Proxy)
		if err != nil {
			return nil, fmt.Errorf("解析代理 URL '%s' 失败: %w", opts.Proxy, err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
		fmt.Printf("提示：使用代理 %s\n", opts.Proxy) // 提示用户正在使用代理
	}

	client := &http.Client{
		Timeout:   time.Second * time.Duration(opts.Timeout),
		Transport: transport,
		// 防止无限重定向
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	return client, nil
}
