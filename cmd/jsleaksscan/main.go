package main

import (
	"fmt"
	"jsleaksscan/internal/config" // 导入配置包
	"jsleaksscan/internal/rules"  // 导入规则包
	"jsleaksscan/internal/scan"   // 导入扫描逻辑包
	"os"
	"runtime"
	"time"
)

func main() {
	// 记录开始时间
	startTime := time.Now()
	fmt.Printf("JsLeaksScan starting at %s...\n", startTime.Format(time.RFC3339))
	fmt.Printf("Detected %d CPU cores.\n", runtime.NumCPU())

	// --- 1. 解析命令行参数 ---
	cfg, err := config.ParseFlags()
	if err != nil {
		// ParseFlags 内部已经处理了打印帮助信息和错误信息
		os.Exit(1)
	}

	// 如果是静默模式，后续很多提示信息将不显示
	if cfg.Quiet {
		// 可以考虑重定向标准输出到 /dev/null 或 NUL
		// 但保留标准错误输出用于显示错误
	}

	if !cfg.Quiet {
		fmt.Printf("运行模式: %s\n", cfg.Mode)
		fmt.Printf("配置文件: %s\n", cfg.ConfigFile)
		fmt.Printf("输出目录: %s\n", cfg.OutputDir)
		if cfg.Mode == "localScan" {
			fmt.Printf("扫描目录: %s\n", cfg.LocalDir)
			fmt.Printf("并发度 (文件处理): %d\n", cfg.ThreadNum)
		} else if cfg.Mode == "urlScan" {
			if cfg.SingleURL != "" {
				fmt.Printf("扫描 URL: %s\n", cfg.SingleURL)
			} else {
				fmt.Printf("URL 文件: %s\n", cfg.URLListFile)
			}
			fmt.Printf("并发度 (URL 请求): %d\n", cfg.ThreadNum)
			fmt.Printf("请求超时: %d 秒\n", cfg.ScanOptions.Timeout)
			if cfg.ScanOptions.Proxy != "" {
				fmt.Printf("使用代理: %s\n", cfg.ScanOptions.Proxy)
			}
			// 可以添加打印其他 URL 扫描选项，如 Header, Method 等，如果 Verbose 开启
			if cfg.Verbose {
				fmt.Printf("  请求方法: %s\n", cfg.ScanOptions.Method)
				if cfg.ScanOptions.Header != "" {
					fmt.Printf("  自定义 Header: %s\n", cfg.ScanOptions.Header)
				}
				if cfg.ScanOptions.Cookie != "" {
					fmt.Printf("  自定义 Cookie: %s\n", cfg.ScanOptions.Cookie)
				}
				// ... 其他选项
			}
		}
	}

	// --- 2. 读取并编译规则 ---
	if !cfg.Quiet {
		fmt.Println("正在加载和编译规则...")
	}
	ruleJsonStr, err := config.ReadConfigFile(cfg.ConfigFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}

	compiledRules, err := rules.CompileRules(ruleJsonStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 编译规则失败: %v\n", err)
		os.Exit(1)
	}
	if compiledRules == nil || (len(compiledRules.Regex) == 0 && len(compiledRules.Literal) == 0) {
		fmt.Fprintln(os.Stderr, "错误: 配置文件中没有加载到有效的规则。请检查配置文件内容。")
		os.Exit(1)
	}
	if !cfg.Quiet {
		fmt.Printf("规则加载完成: %d 正则表达式, %d 字面量\n", len(compiledRules.Regex), len(compiledRules.Literal))
	}

	// --- 3. 执行扫描 ---
	var scanErr error
	switch cfg.Mode {
	case "localScan":
		scanErr = scan.ScanLocalDirectory(cfg, compiledRules)
	case "urlScan":
		scanErr = scan.ScanURLs(cfg, compiledRules)
	default:
		// 此处理论上不会到达，因为 ParseFlags 已经校验过 Mode
		fmt.Fprintf(os.Stderr, "错误: 未知的扫描模式 '%s'\n", cfg.Mode)
		os.Exit(1)
	}

	// 处理扫描过程中可能发生的错误
	if scanErr != nil {
		fmt.Fprintf(os.Stderr, "\n扫描过程中发生错误: %v\n", scanErr)
		// 可以选择在这里退出，或者继续执行后续步骤（如打印总时间）
		// os.Exit(1)
	}

	// --- 4. 结束与总结 ---
	duration := time.Since(startTime)
	fmt.Printf("\n所有扫描任务完成。总执行时间: %v\n", duration)

	// 如果有错误发生，以非零状态退出
	if scanErr != nil {
		os.Exit(1)
	}
}
