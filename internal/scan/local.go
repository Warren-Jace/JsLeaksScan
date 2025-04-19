package scan

import (
	"fmt"
	"io"
	"jsleaksscan/internal/config"
	"jsleaksscan/internal/rules"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ScanLocalDirectory 启动本地目录扫描
func ScanLocalDirectory(cfg *config.AppConfig, compiledRules *rules.CompiledRules) error {
	startTime := time.Now()
	fmt.Printf("开始本地扫描目录: %s (并发度: %d)\n", cfg.LocalDir, cfg.ThreadNum)

	// 检查目录是否存在
	if _, err := os.Stat(cfg.LocalDir); os.IsNotExist(err) {
		return fmt.Errorf("错误: 目录 '%s' 不存在", cfg.LocalDir)
	}

	// 使用信号量控制并发处理文件的数量
	workerSemaphore := make(chan struct{}, cfg.ThreadNum)
	var wg sync.WaitGroup

	// 文件路径通道
	fileQueue := make(chan string, cfg.ThreadNum*2) // 缓冲区大小

	// 启动文件处理 workers
	for i := 0; i < cfg.ThreadNum; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			if !cfg.Quiet && cfg.Verbose {
				fmt.Printf("[Worker %d] 启动\n", workerID)
			}
			for filePath := range fileQueue {
				workerSemaphore <- struct{}{} // 获取一个信号量槽位
				if !cfg.Quiet && cfg.Verbose {
					fmt.Printf("[Worker %d] 开始处理: %s\n", workerID, filePath)
				}
				processLocalFile(filePath, cfg, compiledRules)
				if !cfg.Quiet && cfg.Verbose {
					fmt.Printf("[Worker %d] 完成处理: %s\n", workerID, filePath)
				}
				<-workerSemaphore // 释放信号量槽位
			}
			if !cfg.Quiet && cfg.Verbose {
				fmt.Printf("[Worker %d] 退出\n", workerID)
			}
		}(i)
	}

	// --- 遍历目录并将符合条件的文件放入队列 ---
	// 使用 WaitGroup 确保 Walk 完成后再关闭 fileQueue
	var walkWg sync.WaitGroup
	walkWg.Add(1)
	go func() {
		defer walkWg.Done()
		err := filepath.Walk(cfg.LocalDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				// 打印访问错误并继续遍历其他文件
				fmt.Printf("警告: 访问路径 '%s' 出错: %v\n", path, err)
				return nil // 继续遍历
			}

			// 跳过目录
			if info.IsDir() {
				return nil
			}

			// 检查文件是否符合扫描条件
			if shouldScanFile(path, info) {
				fileQueue <- path // 将文件路径发送到队列
			} else if !cfg.Quiet && cfg.Verbose {
				fmt.Printf("跳过文件 (不符合条件): %s\n", path)
			}
			return nil
		})
		if err != nil {
			fmt.Printf("错误: 遍历目录 '%s' 时发生错误: %v\n", cfg.LocalDir, err)
			// 即使遍历出错，也尝试关闭队列，让 worker 退出
		}
	}()

	// 等待 Walk 完成后关闭文件队列
	go func() {
		walkWg.Wait()
		close(fileQueue)
		if !cfg.Quiet && cfg.Verbose {
			fmt.Println("文件遍历完成，已关闭文件队列。")
		}
	}()

	// 等待所有 worker 完成处理
	wg.Wait()

	fmt.Printf("本地扫描完成。总耗时: %v\n", time.Since(startTime))
	return nil
}

// processLocalFile 读取并处理单个本地文件
func processLocalFile(filePath string, cfg *config.AppConfig, compiledRules *rules.CompiledRules) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("错误: 读取文件 '%s' 失败: %v\n", filePath, err)
		return
	}

	// 如果文件为空，则跳过处理
	if len(content) == 0 {
		if !cfg.Quiet && cfg.Verbose {
			fmt.Printf("跳过空文件: %s\n", filePath)
		}
		return
	}

	// 使用通用内容处理函数
	// 本地扫描通常文件较大，可以考虑默认开启并发正则匹配
	results := processContent(filePath, content, compiledRules, true)

	if len(results) > 0 {
		outputFilePath := GetOutputFilePath(cfg.OutputDir, filePath)
		if err := WriteResultsToFile(outputFilePath, results); err != nil {
			fmt.Printf("错误: 写入结果到 '%s' 失败: %v\n", outputFilePath, err)
		} else {
			if !cfg.Quiet { // 在非静默模式下报告写入成功
				fmt.Printf("发现敏感信息 [%s] -> %s\n", filePath, outputFilePath)
			}
		}
	} else if !cfg.Quiet && cfg.Verbose {
		fmt.Printf("文件 '%s' 未发现匹配项。\n", filePath)
	}
}

// shouldScanFile 判断一个本地文件是否应该被扫描
func shouldScanFile(path string, info os.FileInfo) bool {
	// 1. 基于文件扩展名 (常见脚本和文本文件)
	jsExtensions := map[string]bool{
		".js":   true,
		".jsx":  true,
		".ts":   true,
		".tsx":  true,
		".html": true,
		".htm":  true,
		".json": true,
		".yaml": true,
		".yml":  true,
		".xml":  true,
		".txt":  true,
		".log":  true,
		".conf": true,
		".cfg":  true,
		".ini":  true,
		".md":   true,
		".py":   true, // 添加其他可能包含敏感信息的脚本或配置文件类型
		".sh":   true,
		".rb":   true,
		".php":  true,
		".go":   true, // 扫描 Go 源码本身
		".java": true,
		".cs":   true,
	}
	ext := strings.ToLower(filepath.Ext(path))
	if jsExtensions[ext] {
		return true
	}

	// 2. 基于文件大小 (避免扫描过大的二进制文件)
	// 可根据需要调整大小限制
	maxSize := int64(50 * 1024 * 1024) // 50MB
	if info.Size() > maxSize {
		// fmt.Printf("Skipping large file: %s (size: %d MB)\n", path, info.Size()/(1024*1024))
		return false
	}
	// 对于没有明确扩展名或未知扩展名的文件，可以尝试读取文件头判断 MIME 类型
	// 只有当文件较小且扩展名不明确时才进行 MIME 检测，以提高效率
	if ext == "" || !jsExtensions[ext] && info.Size() < 1*1024*1024 { // 小于 1MB 才检测 MIME
		file, err := os.Open(path)
		if err != nil {
			// fmt.Printf("Warning: Could not open file %s for MIME type detection: %v\n", path, err)
			return false // 打开失败，不扫描
		}
		defer file.Close()

		// 读取文件头部一小部分用于检测
		buffer := make([]byte, 512)
		n, readErr := file.Read(buffer)
		if readErr != nil && readErr != io.EOF {
			// fmt.Printf("Warning: Error reading file %s for MIME type detection: %v\n", path, readErr)
			return false // 读取错误，不扫描
		}

		if n > 0 {
			// 检测 Content-Type
			mimeType := http.DetectContentType(buffer[:n])
			// 常见的文本相关 MIME 类型
			textMimeTypes := map[string]bool{
				"text/plain":               true,
				"text/html":                true,
				"application/javascript":   true,
				"application/json":         true,
				"application/xml":          true,
				"application/x-yaml":       true,  // YAML
				"application/octet-stream": false, // 通常是二进制，但有时也可能是未知文本
				// 可以根据需要添加更多 MIME 类型
			}
			// 去掉 charset 等参数部分
			mimeBase := strings.Split(mimeType, ";")[0]
			if textMimeTypes[mimeBase] {
				return true
			}
			// 特殊处理：如果 MIME 是 octet-stream 但扩展名是已知的文本类型，也扫描
			if mimeBase == "application/octet-stream" && jsExtensions[ext] {
				return true
			}
		}
	}

	return false // 默认不扫描
}
