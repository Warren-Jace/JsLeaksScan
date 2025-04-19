package scan

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"jsleaksscan/internal/config"
	"jsleaksscan/internal/httpclient"
	"jsleaksscan/internal/rules"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// ScanURLs 启动 URL 扫描
func ScanURLs(cfg *config.AppConfig, compiledRules *rules.CompiledRules) error {
	startTime := time.Now()

	// 创建 HTTP 客户端
	client, err := httpclient.CreateHTTPClient(cfg.ScanOptions)
	if err != nil {
		return fmt.Errorf("创建 HTTP 客户端失败: %w", err)
	}

	// 准备 URL 列表
	urlsToScan := []string{}
	if cfg.SingleURL != "" {
		urlsToScan = append(urlsToScan, strings.TrimSpace(cfg.SingleURL))
		fmt.Printf("开始扫描单个 URL: %s (并发度: 1)\n", cfg.SingleURL)
		cfg.ThreadNum = 1 // 单个 URL 不需要高并发
	} else if cfg.URLListFile != "" {
		fmt.Printf("开始从文件扫描 URL: %s (并发度: %d)\n", cfg.URLListFile, cfg.ThreadNum)
		fileURLs, err := readURLsFromFile(cfg.URLListFile)
		if err != nil {
			return fmt.Errorf("读取 URL 文件 '%s' 失败: %w", cfg.URLListFile, err)
		}
		if len(fileURLs) == 0 {
			fmt.Println("警告: URL 文件为空，没有 URL 需要扫描。")
			return nil
		}
		urlsToScan = fileURLs
		fmt.Printf("从文件 '%s' 加载了 %d 个 URL。\n", cfg.URLListFile, len(urlsToScan))
	} else {
		//理论上 config 解析时已处理此情况，但作为防御性编程
		return fmt.Errorf("内部错误：缺少 URL 来源 (既无单个 URL 也无 URL 文件)")
	}

	// 使用 WaitGroup 和信号量控制并发
	var wg sync.WaitGroup
	urlSemaphore := make(chan struct{}, cfg.ThreadNum)
	processedCount := 0
	var countMutex sync.Mutex // 保护 processedCount

	// 遍历 URL 并启动 goroutine 处理
	totalURLs := len(urlsToScan)
	for _, u := range urlsToScan {
		if u == "" { // 跳过空行
			countMutex.Lock()
			processedCount++
			countMutex.Unlock()
			continue
		}
		wg.Add(1)
		urlSemaphore <- struct{}{} // 获取信号量
		go func(targetURL string) {
			defer func() {
				<-urlSemaphore // 释放信号量
				wg.Done()
				countMutex.Lock()
				processedCount++
				if !cfg.Quiet {
					// 打印进度
					fmt.Printf("\r进度: %d/%d (%.2f%%)", processedCount, totalURLs, float64(processedCount)*100/float64(totalURLs))
				}
				countMutex.Unlock()
			}()
			processURL(targetURL, cfg, compiledRules, client)
		}(u)
	}

	// 等待所有 URL 处理完成
	wg.Wait()
	if !cfg.Quiet {
		fmt.Println() // 换行，结束进度条打印
	}
	fmt.Printf("URL 扫描完成。总耗时: %v\n", time.Since(startTime))
	return nil
}

// readURLsFromFile 从文件中读取 URL 列表
func readURLsFromFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var urls []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		url := strings.TrimSpace(scanner.Text())
		if url != "" { // 忽略空行
			urls = append(urls, url)
		}
	}
	return urls, scanner.Err()
}

// processURL 处理单个 URL 的扫描逻辑
func processURL(targetURL string, cfg *config.AppConfig, compiledRules *rules.CompiledRules, client *http.Client) {
	originalURL := targetURL // 保存原始 URL 用于日志和输出

	// 确保 URL 包含协议头
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = "https://" + targetURL // 默认尝试 HTTPS
		if !cfg.Quiet && cfg.Verbose {
			fmt.Printf("URL '%s' 缺少协议，默认使用 https://\n", originalURL)
		}
	}

	// --- 创建 HTTP 请求 ---
	var reqBody io.Reader
	if cfg.ScanOptions.Method == "POST" && cfg.ScanOptions.Data != "" {
		reqBody = strings.NewReader(cfg.ScanOptions.Data)
	}

	req, err := http.NewRequest(cfg.ScanOptions.Method, targetURL, reqBody)
	if err != nil {
		fmt.Printf("错误: 创建请求 '%s' 失败: %v\n", originalURL, err)
		return
	}

	// --- 设置请求头 ---
	// 默认 User-Agent
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36")
	// 其他默认头 (根据需要添加或修改)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate") // 客户端自动处理解压

	// 应用用户自定义或指定的头
	applyCustomHeaders(req, cfg.ScanOptions)

	// --- 执行请求 ---
	if !cfg.Quiet && cfg.Verbose {
		fmt.Printf("正在请求 URL: %s (方法: %s)\n", originalURL, req.Method)
	}

	resp, err := client.Do(req)
	if err != nil {
		// 尝试 HTTP (如果之前是 HTTPS)
		if strings.HasPrefix(targetURL, "https://") && strings.Contains(err.Error(), "http: server gave HTTP response to HTTPS client") {
			targetURL = "http://" + strings.TrimPrefix(targetURL, "https://")
			if !cfg.Quiet && cfg.Verbose {
				fmt.Printf("HTTPS 请求失败，尝试 HTTP: %s\n", targetURL)
			}
			req.URL, _ = req.URL.Parse(targetURL) // 更新请求 URL
			resp, err = client.Do(req)            // 再次尝试
		}

		if err != nil { // 如果仍然有错误
			if !cfg.Quiet { // 只有非静默模式才打印 fetch 错误
				fmt.Printf("错误: 请求 URL '%s' 失败: %v\n", originalURL, err)
			}
			return
		}
	}
	defer resp.Body.Close()

	// --- 检查响应状态码 ---
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if !cfg.Quiet && cfg.Verbose { // 只有 verbose 模式才打印非 2xx 状态码
			fmt.Printf("警告: URL '%s' 返回状态码 %d\n", originalURL, resp.StatusCode)
		}
		// 可以选择性地读取 Body 以获取错误信息，但通常对于扫描目标来说意义不大
		return
	}

	// --- 读取响应体 ---
	// 限制读取大小防止 OOM
	maxBodySize := int64(10 * 1024 * 1024) // 10MB 限制
	limitedReader := io.LimitReader(resp.Body, maxBodySize)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		fmt.Printf("错误: 读取 URL '%s' 响应体失败: %v\n", originalURL, err)
		return
	}

	// 检查是否读取完整 (如果读取量达到限制，说明可能被截断)
	// 再尝试读取一个字节，如果能读到说明超限了
	oneByte := make([]byte, 1)
	n, _ := resp.Body.Read(oneByte) // 尝试从原始 Body 读取
	if n > 0 {
		fmt.Printf("警告: URL '%s' 的响应体超过 %dMB 限制，只处理了部分内容。\n", originalURL, maxBodySize/(1024*1024))
	}

	if len(bodyBytes) == 0 {
		if !cfg.Quiet && cfg.Verbose {
			fmt.Printf("URL '%s' 响应体为空。\n", originalURL)
		}
		return
	}

	// --- 处理内容 ---
	// URL 扫描通常涉及网络 IO，并发正则可能帮助不大，除非响应体特别大
	results := processContent(originalURL, bodyBytes, compiledRules, false)

	// --- 写入结果 ---
	if len(results) > 0 {
		outputFilePath := GetOutputFilePath(cfg.OutputDir, originalURL)
		if err := WriteResultsToFile(outputFilePath, results); err != nil {
			fmt.Printf("错误: 写入结果到 '%s' 失败: %v\n", outputFilePath, err)
		} else {
			if !cfg.Quiet {
				fmt.Printf("发现敏感信息 [%s] -> %s\n", originalURL, outputFilePath)
			}
		}
	} else if !cfg.Quiet && cfg.Verbose {
		fmt.Printf("URL '%s' 未发现匹配项。\n", originalURL)
	}
}

// applyCustomHeaders 将配置中的 Header, Cookie, Auth 等应用到请求对象
func applyCustomHeaders(req *http.Request, opts config.ScanOptions) {
	// 自定义 Header (-H)
	if opts.Header != "" {
		// 尝试解析为 JSON
		var headers map[string]string
		if err := json.Unmarshal([]byte(opts.Header), &headers); err == nil {
			for key, value := range headers {
				req.Header.Set(key, value)
			}
		} else {
			// 尝试解析为 Key:Value,Key2:Value2 格式
			pairs := strings.Split(opts.Header, ",")
			for _, pair := range pairs {
				parts := strings.SplitN(pair, ":", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])
					if key != "" { // 确保 key 不为空
						req.Header.Set(key, value)
					}
				} else if strings.TrimSpace(pair) != "" { // 处理像 "Header;" 这样的情况
					key := strings.TrimSpace(strings.TrimSuffix(pair, ";"))
					if key != "" {
						req.Header.Set(key, "") // 设置空值的 Header
					}
				}
			}
		}
	}

	// User-Agent (--ua)
	if opts.UserAgent != "" {
		req.Header.Set("User-Agent", opts.UserAgent)
	}

	// Referer (--referer)
	if opts.Referer != "" {
		req.Header.Set("Referer", opts.Referer)
	}

	// Cookie (--cookie)
	if opts.Cookie != "" {
		req.Header.Set("Cookie", opts.Cookie)
	}

	// Basic Auth (--auth)
	if opts.Auth != "" {
		// 期望格式是 "user:pass"
		authEncoded := base64.StdEncoding.EncodeToString([]byte(opts.Auth))
		req.Header.Set("Authorization", "Basic "+authEncoded)
	}
}
