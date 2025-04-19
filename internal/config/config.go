package config

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
)

// AppConfig 存储整个应用程序的配置，包括模式和扫描选项
type AppConfig struct {
	Mode        string // "localScan" or "urlScan"
	ConfigFile  string
	OutputDir   string
	ThreadNum   int
	LocalDir    string // Only for localScan
	URLListFile string // Only for urlScan
	SingleURL   string // Only for urlScan
	Verbose     bool
	Quiet       bool
	Help        bool
	ScanOptions ScanOptions // 嵌套扫描选项
	MaxWorkers  int         // 用于本地扫描的 worker 数量
}

// ScanOptions 存储与扫描过程（特别是URL扫描）相关的选项
type ScanOptions struct {
	Proxy     string
	Header    string
	Method    string
	Data      string
	Cookie    string
	Referer   string
	UserAgent string
	Auth      string // "user:pass" format
	Timeout   int    // seconds
}

// ParseFlags 解析命令行参数并返回 AppConfig
func ParseFlags() (*AppConfig, error) {
	cfg := &AppConfig{
		// 设置默认值
		ScanOptions: ScanOptions{
			Method:  "GET",
			Timeout: 10,
		},
		ConfigFile: "config.json",
		OutputDir:  "results",
		ThreadNum:  50,                   // 默认 URL 扫描线程数
		MaxWorkers: runtime.NumCPU() * 2, // 默认本地扫描 worker 数
	}

	// --- 基本选项 ---
	flag.BoolVar(&cfg.Help, "h", false, "显示帮助信息")
	flag.BoolVar(&cfg.Help, "help", false, "显示帮助信息")
	flag.StringVar(&cfg.ConfigFile, "c", cfg.ConfigFile, "配置文件路径")
	flag.StringVar(&cfg.OutputDir, "od", cfg.OutputDir, "结果输出目录")
	flag.StringVar(&cfg.OutputDir, "outputDir", cfg.OutputDir, "结果输出目录") // 长选项名
	flag.IntVar(&cfg.ThreadNum, "t", cfg.ThreadNum, "并发线程数 (URL扫描模式) / 文件处理并发度 (本地扫描模式)")
	flag.BoolVar(&cfg.Verbose, "v", false, "启用详细输出")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "启用详细输出")
	flag.BoolVar(&cfg.Quiet, "q", false, "启用静默模式 (覆盖详细模式)")
	flag.BoolVar(&cfg.Quiet, "quiet", false, "启用静默模式")

	// --- 本地扫描特定选项 ---
	flag.StringVar(&cfg.LocalDir, "d", "", "本地扫描模式: 包含要扫描文件的目录路径")
	flag.StringVar(&cfg.LocalDir, "dirname", "", "本地扫描模式: 包含要扫描文件的目录路径")

	// --- URL 扫描特定选项 ---
	flag.StringVar(&cfg.URLListFile, "uf", "", "URL扫描模式: 包含要扫描URL列表的文件路径")
	flag.StringVar(&cfg.URLListFile, "urlFileName", "", "URL扫描模式: 包含要扫描URL列表的文件路径")
	flag.StringVar(&cfg.SingleURL, "u", "", "URL扫描模式: 直接扫描单个URL")
	flag.StringVar(&cfg.SingleURL, "url", "", "URL扫描模式: 直接扫描单个URL")
	flag.StringVar(&cfg.ScanOptions.Proxy, "p", "", "URL扫描模式: 代理设置 (例如: http://127.0.0.1:8080)")
	flag.StringVar(&cfg.ScanOptions.Proxy, "proxy", "", "URL扫描模式: 代理设置")
	flag.StringVar(&cfg.ScanOptions.Header, "H", "", "URL扫描模式: 自定义HTTP头 (例如: \"Key:Value\" 或 JSON)")
	flag.StringVar(&cfg.ScanOptions.Header, "header", "", "URL扫描模式: 自定义HTTP头")
	flag.StringVar(&cfg.ScanOptions.Method, "m", cfg.ScanOptions.Method, "URL扫描模式: HTTP请求方法")
	flag.StringVar(&cfg.ScanOptions.Method, "method", cfg.ScanOptions.Method, "URL扫描模式: HTTP请求方法")
	flag.StringVar(&cfg.ScanOptions.Data, "data", "", "URL扫描模式: HTTP请求数据 (POST请求body)")
	flag.StringVar(&cfg.ScanOptions.Cookie, "cookie", "", "URL扫描模式: HTTP请求Cookie")
	flag.StringVar(&cfg.ScanOptions.Referer, "r", "", "URL扫描模式: HTTP请求Referer")
	flag.StringVar(&cfg.ScanOptions.Referer, "referer", "", "URL扫描模式: HTTP请求Referer")
	flag.StringVar(&cfg.ScanOptions.UserAgent, "ua", "", "URL扫描模式: HTTP请求User-Agent (为空则使用默认值)")
	flag.StringVar(&cfg.ScanOptions.UserAgent, "userAgent", "", "URL扫描模式: HTTP请求User-Agent")
	flag.StringVar(&cfg.ScanOptions.Auth, "a", "", "URL扫描模式: HTTP Basic Auth认证 (格式: user:pass)")
	flag.StringVar(&cfg.ScanOptions.Auth, "auth", "", "URL扫描模式: HTTP Basic Auth认证")
	flag.IntVar(&cfg.ScanOptions.Timeout, "timeout", cfg.ScanOptions.Timeout, "URL扫描模式: 请求超时时间(秒)")

	// 自定义 Usage
	flag.Usage = func() { ShowHelp("") } // 默认显示通用帮助

	// --- 解析模式 ---
	// 我们需要先确定模式，因为帮助信息依赖于模式
	args := os.Args[1:] // 获取除程序名外的所有参数
	mode := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		// 第一个参数不是 flag，认为是 mode
		mode = args[0]
		args = args[1:] // 从参数列表中移除 mode
	}

	// 解析剩余的参数
	flag.CommandLine.Parse(args)

	// 处理帮助请求
	if cfg.Help {
		ShowHelp(mode) // 显示特定模式或通用帮助
		os.Exit(0)
	}

	// 设置并验证模式
	if mode == "localScan" {
		cfg.Mode = "localScan"
		if cfg.LocalDir == "" {
			return nil, fmt.Errorf("错误：本地扫描模式 (localScan) 需要指定目录 (-d/--dirname)")
		}
		if cfg.SingleURL != "" || cfg.URLListFile != "" {
			fmt.Println("警告：在 localScan 模式下，URL 相关参数 (-u, -uf) 将被忽略。")
		}
		// 本地扫描模式下，线程数可以基于 CPU 核数调整，如果用户未指定 -t
		if !isFlagPassed("t") { // 检查用户是否显式设置了 -t
			cfg.ThreadNum = cfg.MaxWorkers
			if !cfg.Quiet {
				fmt.Printf("提示：本地扫描模式未指定 -t，使用默认并发度: %d (CPU核心数 * 2)\n", cfg.ThreadNum)
			}
		}

	} else if mode == "urlScan" {
		cfg.Mode = "urlScan"
		if (cfg.SingleURL == "" && cfg.URLListFile == "") || (cfg.SingleURL != "" && cfg.URLListFile != "") {
			return nil, fmt.Errorf("错误：URL扫描模式 (urlScan) 需要且仅需要指定一个 URL 源 (-u/--url 或 -uf/--urlFileName)")
		}
		if cfg.LocalDir != "" {
			fmt.Println("警告：在 urlScan 模式下，本地目录参数 (-d) 将被忽略。")
		}
	} else if mode != "" {
		return nil, fmt.Errorf("错误：无法识别的模式 '%s'。有效模式为 'localScan' 或 'urlScan'", mode)
	} else {
		// 没有指定模式
		if cfg.LocalDir != "" { // 如果指定了 -d，则推断为 localScan
			cfg.Mode = "localScan"
			fmt.Println("提示：未明确指定模式，但提供了 -d 参数，假设为 localScan 模式。")
		} else if cfg.SingleURL != "" || cfg.URLListFile != "" { // 如果指定了 URL 源，则推断为 urlScan
			cfg.Mode = "urlScan"
			fmt.Println("提示：未明确指定模式，但提供了 URL 参数 (-u 或 -uf)，假设为 urlScan 模式。")
			// 再次检查 URL 源的互斥性
			if (cfg.SingleURL == "" && cfg.URLListFile == "") || (cfg.SingleURL != "" && cfg.URLListFile != "") {
				return nil, fmt.Errorf("错误：URL扫描模式 (urlScan) 需要且仅需要指定一个 URL 源 (-u/--url 或 -uf/--urlFileName)")
			}
		} else {
			// 既没有模式，也没有能推断模式的参数
			ShowHelp("")
			return nil, fmt.Errorf("错误：必须指定扫描模式 (localScan 或 urlScan) 或提供可推断模式的参数 (-d, -u, -uf)")
		}
	}

	// 验证配置文件是否存在
	if _, err := os.Stat(cfg.ConfigFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("错误: 配置文件 '%s' 不存在", cfg.ConfigFile)
	}

	// 创建输出目录
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("错误: 创建输出目录 '%s' 失败: %w", cfg.OutputDir, err)
	}

	return cfg, nil
}

// ReadConfigFile 读取配置文件内容
func ReadConfigFile(configPath string) (string, error) {
	byteValue, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("读取配置文件 '%s' 失败: %w", configPath, err)
	}
	return string(byteValue), nil
}

// ShowHelp 显示帮助信息
func ShowHelp(mode string) {
	fmt.Fprintf(os.Stderr, `JsLeaksScan - JavaScript 敏感信息扫描工具

Usage:
  jsleaksscan <mode> [options]

模式 (Mode):
  localScan       扫描本地文件系统中的文件
  urlScan         扫描在线的 URL

基本选项 (适用于所有模式):
`)
	printDefaults("c", "od", "t", "v", "q", "h") // 打印通用选项

	if mode == "localScan" || mode == "" { // 显示 localScan 或通用帮助时
		fmt.Fprintf(os.Stderr, `
本地扫描模式 (localScan) 选项:
`)
		printDefaults("d")
	}

	if mode == "urlScan" || mode == "" { // 显示 urlScan 或通用帮助时
		fmt.Fprintf(os.Stderr, `
在线扫描模式 (urlScan) 选项:
`)
		printDefaults("u", "uf", "p", "H", "m", "data", "cookie", "r", "ua", "a", "timeout")
	}

	fmt.Fprintf(os.Stderr, `
示例:
  # 扫描本地目录 'js_files' (结果写入 results/ 目录)
  jsleaksscan localScan -d js_files/ -c config.json -t %d

  # 扫描 'urls.txt' 文件中的 URL (结果写入 results/ 目录, 每个 URL 一个文件)
  jsleaksscan urlScan -uf urls.txt -c config.json -t 50 -p http://127.0.0.1:8080

  # 扫描单个 URL
  jsleaksscan urlScan -u https://example.com/main.js -c config.json

`, runtime.NumCPU()*2) // 在示例中显示默认本地线程数
}

// printDefaults 辅助函数，用于打印特定 flag 的默认值和用法
func printDefaults(names ...string) {
	printed := make(map[string]bool)
	flag.VisitAll(func(f *flag.Flag) {
		for _, name := range names {
			if f.Name == name && !printed[f.Name] {
				// 尝试找到长短选项名对
				longName := ""
				shortName := ""
				if len(f.Name) == 1 {
					shortName = "-" + f.Name
					// 尝试查找对应的长选项名
					flag.VisitAll(func(f2 *flag.Flag) {
						if len(f2.Name) > 1 && f2.Usage == f.Usage && f2.DefValue == f.DefValue {
							longName = "--" + f2.Name
						}
					})
				} else {
					longName = "--" + f.Name
					// 尝试查找对应的短选项名
					flag.VisitAll(func(f2 *flag.Flag) {
						if len(f2.Name) == 1 && f2.Usage == f.Usage && f2.DefValue == f.DefValue {
							shortName = "-" + f2.Name
						}
					})
				}

				nameStr := ""
				if shortName != "" && longName != "" {
					nameStr = fmt.Sprintf("  %s, %s", shortName, longName)
					printed[strings.TrimPrefix(longName, "--")] = true // 标记长名已打印
				} else if longName != "" {
					nameStr = fmt.Sprintf("      %s", longName)
				} else {
					nameStr = fmt.Sprintf("  %s", shortName)
				}

				// 添加类型信息（对非 bool 类型）
				typeName := ""
				if _, ok := f.Value.(flag.Getter).Get().(bool); !ok {
					typeName = fmt.Sprintf(" <%T>", f.Value.(flag.Getter).Get())
					// 简化类型名
					typeName = strings.Replace(typeName, " <int>", " <int>", 1)
					typeName = strings.Replace(typeName, " <string>", " <string>", 1)
				}

				fmt.Fprintf(os.Stderr, "%-25s %s", nameStr+typeName, f.Usage)
				// 只为非 bool 且有默认值的 flag 显示默认值
				if typeName != "" && f.DefValue != "" && f.DefValue != "0" {
					fmt.Fprintf(os.Stderr, " (默认: %q)", f.DefValue)
				}
				fmt.Fprintln(os.Stderr)
				printed[f.Name] = true // 标记已打印
				break                  // 处理完一个名字就跳出内层循环
			}
		}
	})
}

// isFlagPassed 检查某个 flag 是否在命令行中被显式设置
func isFlagPassed(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}
