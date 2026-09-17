package query

import (
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/freezxp/syslogx/backend/internal/storage"
	"strconv"
	"strings"
	"time"
)

var allowedOps = map[string]bool{"eq": true, "neq": true, "contains": true, "starts_with": true, "exists": true, "gt": true, "gte": true, "lt": true, "lte": true}

func Validate(q *storage.Query) error {
	if q.Start.IsZero() || q.End.IsZero() || !q.Start.Before(q.End) {
		return errors.New("start must be before end")
	}
	if q.End.Sub(q.Start) > 31*24*time.Hour {
		return errors.New("query range exceeds 31 days")
	}
	if q.Limit == 0 {
		q.Limit = 100
	}
	if q.Limit < 1 || q.Limit > 1000 {
		return errors.New("limit must be between 1 and 1000")
	}
	if len(q.Text) > 4096 || len(q.Native) > 8192 || len(q.Filters) > 32 {
		return errors.New("query is too large")
	}
	for _, f := range q.Filters {
		if !ValidField(f.Field) || !allowedOps[f.Op] {
			return errors.New("invalid filter")
		}
	}
	return nil
}
func LogsQL(q storage.Query) (string, error) {
	if err := Validate(&q); err != nil {
		return "", err
	}
	base := fmt.Sprintf("_time:[%s, %s] tenant_id:%s", q.Start.UTC().Format(time.RFC3339Nano), q.End.UTC().Format(time.RFC3339Nano), quote(q.TenantID))
	if q.Native != "" {
		base += " " + q.Native
	} else if q.Text != "" {
		base += " " + quote(q.Text)
	}
	for _, f := range q.Filters {
		field, val := f.Field, quote(fmt.Sprint(f.Value))
		switch f.Op {
		case "eq":
			base += " " + field + ":" + val
		case "neq":
			base += " NOT " + field + ":" + val
		case "contains":
			base += " " + field + ":~" + val
		case "starts_with":
			base += " " + field + ":^" + val
		case "exists":
			base += " " + field + ":*"
		case "gt", "gte", "lt", "lte":
			base += " " + field + f.Op + val
		}
	}
	if q.Cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(q.Cursor)
		if err != nil {
			return "", errors.New("invalid cursor")
		}
		base += " _time:<" + quote(string(raw))
	}
	return base, nil
}
func Cursor(t string) string { return base64.RawURLEncoding.EncodeToString([]byte(t)) }
func quote(v string) string  { return strconv.Quote(strings.ReplaceAll(v, "\x00", "")) }
func ValidField(v string) bool {
	if v == "" || len(v) > 128 {
		return false
	}
	for _, r := range v {
		if !(r == '_' || r == '.' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}
