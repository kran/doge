package duck

import (
	"fmt"
	"log"
	"strings"
)

func quoter(s string) string {
	if strings.Contains(s, `"`) || s == "*" {
		return s
	}
	return `"` + s + `"`
}

func mysqlQuoter(s string) string {
	if strings.Contains(s, "`") || s == "*" {
		return s
	}
	return "`" + s + "`"
}

func logger(query string, args ...any) {
	log.Printf("\n-> SQL: %s\n-> ARGS: %v\n", query, args)
}

func pgMarker(idx int) string {
	return fmt.Sprintf("$%d", idx+1)
}

func marker(idx int) string {
	return "?"
}
