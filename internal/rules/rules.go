package rules

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// CompiledRules 存储编译后的规则
type CompiledRules struct {
	Regex   map[string]*regexp.Regexp
	Literal map[string]string
}

// JsonToMap 将 JSON 字符串转换为 map[string]string
func JsonToMap(jsonStr string) (map[string]string, error) {
	// 预估 map 大小以提高性能
	estimatedPairs := strings.Count(jsonStr, ":")
	m := make(map[string]string, estimatedPairs)
	// 使用 Decoder 处理可能更健壮
	decoder := json.NewDecoder(strings.NewReader(jsonStr))
	if err := decoder.Decode(&m); err != nil {
		return nil, fmt.Errorf("JSON 解码错误: %w", err)
	}
	return m, nil
}

// isLiteralPattern 检查一个字符串是否可以被视为字面量模式（不包含正则元字符）
// 注意：这个检查可能不完全准确，复杂的字面量可能误判为正则
func isLiteralPattern(pattern string) bool {
	// `\` 需要特殊处理，因为它本身也是元字符
	// `.` `+` `*` `?` `(` `)` `|` `[` `]` `{` `}` `^` `$`
	return !strings.ContainsAny(pattern, ".+*?()|[]{}^$") && !strings.Contains(pattern, `\`)
}

// CompileRules 从 JSON 字符串编译规则
func CompileRules(ruleJsonStr string) (*CompiledRules, error) {
	ruleMap, err := JsonToMap(ruleJsonStr)
	if err != nil {
		return nil, fmt.Errorf("解析规则 JSON 失败: %w", err)
	}

	compiled := &CompiledRules{
		Regex:   make(map[string]*regexp.Regexp),
		Literal: make(map[string]string),
	}

	for name, pattern := range ruleMap {
		if pattern == "" {
			fmt.Printf("警告：规则 '%s' 的模式为空，已跳过。\n", name)
			continue // 跳过空模式
		}
		if isLiteralPattern(pattern) {
			compiled.Literal[name] = pattern
		} else {
			// 尝试编译为正则表达式
			// 为提高性能，可以考虑使用 regexp.MustCompile，但这会在编译失败时 panic
			reg, err := regexp.Compile(pattern)
			if err != nil {
				// 如果编译失败，可以考虑将其视为字面量，或者报错
				fmt.Printf("警告：编译规则 '%s' 的正则表达式 '%s' 失败: %v。将尝试作为字面量处理。\n", name, pattern, err)
				// 或者选择报错并退出：
				// return nil, fmt.Errorf("编译规则 '%s' 的正则表达式失败: %w", name, err)
				compiled.Literal[name] = pattern // 编译失败则视为字面量
			} else {
				compiled.Regex[name] = reg
			}
		}
	}

	fmt.Printf("规则编译完成：加载了 %d 条正则表达式规则，%d 条字面量规则。\n", len(compiled.Regex), len(compiled.Literal))
	return compiled, nil
}
