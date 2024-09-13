package utils

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func Contains(l []string, s string) bool {
	for _, a := range l {
		if a == s {
			return true
		}
	}
	return false
}

func Intersects(a, b []string) bool {
	for _, v := range a {
		if Contains(b, v) {
			return true
		}
	}
	return false
}

func ContainsAndTrimPrefix(name string, prefixes []string) (string, bool) {
	if prefix, ok := ContainsPrefix(name, prefixes); ok {
		return strings.TrimPrefix(name, prefix), true
	}
	return name, false
}

func ContainsPrefix(name string, prefixes []string) (string, bool) {
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			return prefix, true
		}
	}
	return "", false
}

func GetFirstNotNil[T any](t1, t2 *T) *T {
	if t1 != nil {
		return t1
	}
	return t2
}
func LookupInt(key string, def ...int) int {
	val := LookupEnv(key, "")
	if val == "" {
		if len(def) > 0 {
			return def[0]
		}
		return 0
	}
	res, err := strconv.Atoi(val)
	if err != nil {
		panic(fmt.Errorf("%s: %w", key, err))
	}
	return res
}

func LookupEnv(key string, def ...string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	if len(def) > 0 {
		return def[0]
	}
	return ""
}
func LookupDuration(key string, def ...time.Duration) time.Duration {
	if value, ok := os.LookupEnv(key); ok {
		d, err := time.ParseDuration(value)
		if err != nil {
			panic(fmt.Sprintf("cant parse env '%s' as duration :%s", key, err.Error()))
		}
		return d
	}
	if len(def) > 0 {
		return def[0]
	}
	return time.Duration(0)
}

func LookupBool(key string, def ...bool) bool {
	val := LookupEnv(key, "")
	if val == "" {
		if len(def) > 0 {
			return def[0]
		}
		return false
	}
	res, err := strconv.ParseBool(val)
	if err != nil {
		panic(fmt.Errorf("%s: %w", key, err))
	}
	return res
}

func WildcardMatch(pattern, name string) bool {
	return regexp.MustCompile(wildCardToRegexp(pattern)).MatchString(name)
}

func wildCardToRegexp(pattern string) string {
	var result strings.Builder
	for i, literal := range strings.Split(pattern, "*") {

		// Replace * with .*
		if i > 0 {
			result.WriteString(".*")
		}

		// Quote any regular expression meta characters in the
		// literal text.
		result.WriteString(regexp.QuoteMeta(literal))
	}
	return result.String()
}

func GetRepositoryAndTag(image string) (repository, tag string) {
	image = strings.TrimPrefix(image, "/")
	repository, tag, found := strings.Cut(image, ":")
	if !found {
		tag = "latest"
	}
	return
}

func SaveToFile(filename string, data []byte) error {
	fd, err := os.Create(filename)
	if err != nil {
		return err
	}
	_, err = fd.Write(data)
	if err != nil {
		return err
	}
	return nil
}
