package msstrings

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"
)

func JoinStrings(data ...any) string {
	var sBuilder strings.Builder
	for _, datum := range data {
		sBuilder.WriteString(checkString(datum))
	}
	return sBuilder.String()
}

func checkString(v any) string {
	value := reflect.ValueOf(v)
	switch value.Kind() {
	case reflect.String:
		return v.(string)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func SubStringLast(str, substr string) string {
	index := strings.Index(str, substr)
	if index < 0 {
		return ""
	}
	return str[index+len(substr):]
}

func IsASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}
