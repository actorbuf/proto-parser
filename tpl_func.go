package proto_parser

import (
	"crypto/md5"
	"encoding/hex"
	"strings"
)

// Len 返回字符串长度
func Len(s string) int {
	return len([]rune(s))
}

// IsBodyEmpty 是否不需要关心文档返回
func IsBodyEmpty(s string) bool {
	if len(s) == 0 || s == "{}" {
		return true
	}
	return false
}

// NotBodyEmpty 是否文档为空
func NotBodyEmpty(s string) bool {
	if len(s) == 0 || s == "{}" {
		return false
	}
	return true
}

// IsOuterProjectErrorCode 是否是项目外部的错误码
func IsOuterProjectErrorCode(err string) bool {
	return strings.Contains(err, ".")
}

// MD5 md5一下
func MD5(a, b string) string {
	h := md5.New()
	h.Write([]byte(a + "_" + b))
	return hex.EncodeToString(h.Sum(nil))[8:24]
}
