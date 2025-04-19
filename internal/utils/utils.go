package utils

import (
	"bytes"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
)

// 缓冲池初始化
var BufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

// SanitizeFilename 清理文件名，使其安全适用于文件系统
func SanitizeFilename(path string) string {
	// 尝试解析为 URL，提取 Hostname 和 Path
	u, err := url.Parse(path)
	if err == nil && u.Hostname() != "" { // 确保是有效的 URL 且有 Host
		// 替换路径中的斜杠为下划线，并结合 Hostname
		sanitizedPath := u.Hostname() + strings.ReplaceAll(u.Path, "/", "_")
		path = sanitizedPath // 使用清理后的路径作为基础
	} else {
		// 如果不是 URL 或解析失败，则使用原始路径的基础名
		path = filepath.Base(path)
	}

	// 移除或替换非法字符
	sanitized := strings.Map(func(r rune) rune {
		// 允许字母、数字、下划线、连字符、点
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			return r
		}
		// 其他字符替换为下划线
		return '_'
	}, path) // 直接在处理后的 path 上操作

	// 限制文件名最大长度
	maxLength := 200 // 调整一个合理的文件名长度限制
	if len(sanitized) > maxLength {
		sanitized = sanitized[:maxLength]
	}

	// 避免文件名以 '.' 或 '_' 开头
	if len(sanitized) > 0 && (sanitized[0] == '.' || sanitized[0] == '_') {
		sanitized = "file_" + sanitized
	}

	// 处理空文件名的情况
	if sanitized == "" {
		sanitized = "default_filename"
	}

	return sanitized
}

// ResolveRelativeURL 解析相对URL (如果需要的话)
func ResolveRelativeURL(base, relative string) string {
	baseURL, err := url.Parse(base)
	if err != nil {
		return relative // Base URL 无效，返回原始相对 URL
	}

	relURL, err := url.Parse(relative)
	if err != nil {
		return relative // 相对 URL 无效，返回原始相对 URL
	}

	return baseURL.ResolveReference(relURL).String()
}
