package validate

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var uuidRe = regexp.MustCompile(
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`,
)

func UUID(name, value string) error {
	if !uuidRe.MatchString(value) {
		return fmt.Errorf("%s must be a valid UUID", name)
	}
	return nil
}

func StringField(name, value string, maxLen int) (string, error) {
	v := strings.TrimSpace(value)
	if utf8.RuneCountInString(v) > maxLen {
		return "", fmt.Errorf("%s exceeds maximum length of %d characters", name, maxLen)
	}
	return v, nil
}

func Required(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

func MaxBytes(name string, data []byte, maxBytes int) error {
	if len(data) > maxBytes {
		return fmt.Errorf("%s exceeds maximum size of %d bytes", name, maxBytes)
	}
	return nil
}
