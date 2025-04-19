package scan

import (
	"bufio"
	"bytes"
	"fmt"
	"jsleaksscan/internal/rules" // 导入规则包
	"jsleaksscan/internal/utils" // 导入工具包
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

// ScanResult 存储单次扫描发现的结果
type ScanResult struct {
	Source string // 文件路径或 URL
	Rule   string // 命中的规则名
	Match  string // 匹配到的具体内容
}

// WriteResultsToFile 将结果批量写入单个文件
// 使用锁确保并发写入安全
var fileWriteMutex sync.Mutex

func WriteResultsToFile(filename string, results []ScanResult) error {
	if len(results) == 0 {
		return nil // 没有结果，无需写入
	}

	fileWriteMutex.Lock()
	defer fileWriteMutex.Unlock()

	// O_APPEND 模式打开文件，允许多个 goroutine 安全地追加写入
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("打开输出文件 '%s' 失败: %w", filename, err)
	}
	defer file.Close()

	// 预估缓冲区大小
	estimatedSize := 0
	for _, result := range results {
		estimatedSize += len(result.Source) + len(result.Rule) + len(result.Match) + 10 // 估算额外字符
	}
	buf := bytes.NewBuffer(make([]byte, 0, estimatedSize))

	// 格式化结果并写入缓冲区
	for _, result := range results {
		// 格式：[来源] 规则名: 匹配内容
		fmt.Fprintf(buf, "[%s] %s: %s\n", result.Source, result.Rule, result.Match)
	}

	// 使用带缓冲的写入器提高性能
	writer := bufio.NewWriterSize(file, 64*1024) // 64KB buffer
	if _, err := writer.Write(buf.Bytes()); err != nil {
		_ = writer.Flush() // 尝试刷新缓冲区
		return fmt.Errorf("写入结果到 '%s' 失败: %w", filename, err)
	}

	// 确保所有缓冲数据写入文件
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("刷新缓冲区到 '%s' 失败: %w", filename, err)
	}

	return nil
}

// processContent 对给定的内容（字节切片）应用规则集
// sourceIdentifier 用于结果输出，可以是文件路径或 URL
// Returns a slice of ScanResult
func processContent(sourceIdentifier string, content []byte, compiledRules *rules.CompiledRules, useConcurrency bool) []ScanResult {
	var combinedResults []ScanResult

	// 1. 处理字面量规则
	literalMatches := processLiteralRules(sourceIdentifier, content, compiledRules.Literal)
	combinedResults = append(combinedResults, literalMatches...)

	// 2. 处理正则表达式规则
	var regexMatches []ScanResult
	// 根据内容大小和规则数量决定是否并发处理正则
	shouldBeConcurrent := useConcurrency && len(content) > 1024*1024 && len(compiledRules.Regex) > 5
	if shouldBeConcurrent {
		regexMatches = processRegexRulesConcurrently(sourceIdentifier, content, compiledRules.Regex)
	} else {
		regexMatches = processRegexRulesSerially(sourceIdentifier, content, compiledRules.Regex)
	}
	combinedResults = append(combinedResults, regexMatches...)

	return combinedResults
}

// processLiteralRules 处理字面量规则
func processLiteralRules(source string, content []byte, literalRules map[string]string) []ScanResult {
	var results []ScanResult
	patternBytes := utils.BufferPool.Get().(*bytes.Buffer)
	patternBytes.Reset()
	defer utils.BufferPool.Put(patternBytes)

	for ruleName, pattern := range literalRules {
		patternBytes.Reset()
		patternBytes.WriteString(pattern) // 将 pattern 转换为 []byte
		if bytes.Contains(content, patternBytes.Bytes()) {
			results = append(results, ScanResult{
				Source: source,
				Rule:   ruleName,
				Match:  pattern, // 字面量匹配，直接用 pattern 作为匹配内容
			})
		}
	}
	return results
}

// processRegexRulesSerially 串行处理正则表达式规则
func processRegexRulesSerially(source string, content []byte, regexRules map[string]*regexp.Regexp) []ScanResult {
	var results []ScanResult
	buf := utils.BufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer utils.BufferPool.Put(buf)

	for ruleName, reg := range regexRules {
		// FindAllIndex 效率可能更高，因为它避免了子切片的创建
		// -1 表示查找所有匹配项
		matches := reg.FindAll(content, -1)
		for _, match := range matches {
			// 检查匹配是否为空或过长 (可选，防止意外匹配)
			if len(match) > 0 && len(match) < 1024 { // 示例：限制匹配长度
				results = append(results, ScanResult{
					Source: source,
					Rule:   ruleName,
					Match:  string(match), // 需要转换为 string
				})
			}
		}
	}
	return results
}

// processRegexRulesConcurrently 并行处理正则表达式规则
func processRegexRulesConcurrently(source string, content []byte, regexRules map[string]*regexp.Regexp) []ScanResult {
	resultChan := make(chan ScanResult, len(regexRules)*5) // 估算通道大小
	var wg sync.WaitGroup

	for ruleName, reg := range regexRules {
		wg.Add(1)
		go func(name string, regex *regexp.Regexp) {
			defer wg.Done()
			// 每个 goroutine 查找自己的匹配
			matches := regex.FindAll(content, -1)
			for _, match := range matches {
				// 检查匹配是否为空或过长
				if len(match) > 0 && len(match) < 1024 {
					resultChan <- ScanResult{
						Source: source,
						Rule:   name,
						Match:  string(match),
					}
				}
			}
		}(ruleName, reg)
	}

	// 启动一个 goroutine 等待所有规则处理完成，然后关闭通道
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 从通道收集结果
	results := make([]ScanResult, 0, len(resultChan)) // 预估容量
	for result := range resultChan {
		results = append(results, result)
	}

	return results
}

// GetOutputFilePath 生成结果文件的完整路径
func GetOutputFilePath(outputDir, sourceIdentifier string) string {
	sanitized := utils.SanitizeFilename(sourceIdentifier)
	// 如果清理后的文件名没有扩展名，添加 .txt
	if filepath.Ext(sanitized) == "" {
		sanitized += ".txt"
	}
	return filepath.Join(outputDir, sanitized)
}
