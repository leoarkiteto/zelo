package testutil

import (
	"fmt"
	"hash/crc32"
	"net/url"
	"os"
)

// TestRedisURL returns a TEST_REDIS_URL pointing at a per-suffix Redis DB
// number on the same server, so parallel test packages do not share keys.
// It returns "" when TEST_REDIS_URL is unset, matching the repo's existing
// integration-test convention (tests are skipped without it).
func TestRedisURL(suffix string) string {
	base := os.Getenv("TEST_REDIS_URL")
	if base == "" {
		return ""
	}
	u, err := url.Parse(base)
	if err != nil {
		panic(fmt.Sprintf("parse TEST_REDIS_URL: %v", err))
	}
	db := crc32.ChecksumIEEE([]byte(suffix)) % 16
	u.Path = fmt.Sprintf("/%d", db)
	return u.String()
}
