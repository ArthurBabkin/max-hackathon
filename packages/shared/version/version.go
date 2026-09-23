// Package version — коммит, из которого собран бэкенд.
//
// Функции собирает Yandex Cloud из архива исходников, где нет .git, поэтому
// ни -ldflags, ни debug.ReadBuildInfo коммит не дадут. В репозитории файл
// содержит «dev»; при выкладке CI записывает в него короткий хеш коммита.
package version

import (
	_ "embed"
	"strings"
)

//go:embed commit.txt
var commit string

// Commit — короткий хеш коммита или «dev» для локальной сборки.
func Commit() string {
	if c := strings.TrimSpace(commit); c != "" {
		return c
	}
	return "dev"
}
