package migrations

import (
	"embed"
	"fmt"
	"strconv"
	"strings"
)

//go:embed *.sql
var files embed.FS

func Versions() ([]int64, error) {
	entries, err := files.ReadDir(".")
	if err != nil {
		return nil, err
	}
	versions := make([]int64, 0, len(entries))
	for _, entry := range entries {
		prefix, _, ok := strings.Cut(entry.Name(), "_")
		version, err := strconv.ParseInt(prefix, 10, 64)
		if !ok || err != nil || version <= 0 {
			return nil, fmt.Errorf("invalid migration filename: %s", entry.Name())
		}
		versions = append(versions, version)
	}
	return versions, nil
}
