package parser

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var ErrInvalid = errors.New("invalid syslog message")

type Event struct {
	Timestamp         time.Time
	TimestampInferred bool
	Message           string
	Hostname          string
	Facility          *uint8
	Severity          *uint8
	Priority          *uint8
	AppName           string
	ProcessID         string
	MessageID         string
	Format            string
	Fields            map[string]any
}

type Parser interface {
	Name() string
	Parse(context.Context, []byte, time.Time, *time.Location) (Event, error)
}

type Registry struct{ parsers map[string]Parser }

func NewRegistry() *Registry {
	items := []Parser{RFC3164{}, RFC5424{}}
	r := &Registry{parsers: make(map[string]Parser, len(items))}
	for _, p := range items {
		r.parsers[p.Name()] = p
	}
	return r
}

func (r *Registry) Parse(ctx context.Context, configured string, payload []byte, received time.Time, loc *time.Location) (Event, error) {
	name := configured
	if name == "" || name == "auto" {
		name = Detect(payload)
		if name == "unknown" {
			return Event{Timestamp: received.UTC(), TimestampInferred: true, Message: strings.ToValidUTF8(string(payload), "�"), Format: "unknown", Fields: map[string]any{}}, nil
		}
	}
	p, ok := r.parsers[name]
	if !ok {
		return Event{}, fmt.Errorf("parser %q not registered", name)
	}
	return p.Parse(ctx, payload, received, loc)
}

func Detect(payload []byte) string {
	s := string(payload)
	end := strings.IndexByte(s, '>')
	if len(s) < 4 || s[0] != '<' || end < 2 || end > 4 {
		return "unknown"
	}
	if _, err := strconv.Atoi(s[1:end]); err != nil {
		return "unknown"
	}
	rest := s[end+1:]
	if len(rest) >= 2 && rest[0] >= '1' && rest[0] <= '9' && rest[1] == ' ' {
		return "rfc5424"
	}
	if rfc3164Pattern.MatchString(s) {
		return "rfc3164"
	}
	return "unknown"
}

func priority(value string) (p, facility, severity *uint8, err error) {
	n, parseErr := strconv.Atoi(value)
	if parseErr != nil || n < 0 || n > 191 {
		return nil, nil, nil, fmt.Errorf("%w: priority", ErrInvalid)
	}
	pv, fv, sv := uint8(n), uint8(n/8), uint8(n%8)
	return &pv, &fv, &sv, nil
}

var rfc3164Pattern = regexp.MustCompile(`^<(\d{1,3})>([A-Z][a-z]{2})\s+([ 0-9]\d?)\s+(\d{2}:\d{2}:\d{2})\s+(\S+)\s*(.*)$`)

type RFC3164 struct{}

func (RFC3164) Name() string { return "rfc3164" }

func (RFC3164) Parse(_ context.Context, payload []byte, received time.Time, loc *time.Location) (Event, error) {
	m := rfc3164Pattern.FindStringSubmatch(strings.TrimRight(string(payload), "\r\n"))
	if m == nil {
		return Event{}, fmt.Errorf("%w: RFC3164 header", ErrInvalid)
	}
	p, facility, severity, err := priority(m[1])
	if err != nil {
		return Event{}, err
	}
	stamp, err := nearestRFC3164Time(m[2], strings.TrimSpace(m[3]), m[4], received, loc)
	if err != nil {
		return Event{}, fmt.Errorf("%w: timestamp: %v", ErrInvalid, err)
	}
	app, pid, msg := parseTag(m[6])
	return Event{Timestamp: stamp.UTC(), TimestampInferred: true, Message: msg, Hostname: m[5], Facility: facility, Severity: severity, Priority: p, AppName: app, ProcessID: pid, Format: "rfc3164", Fields: map[string]any{}}, nil
}

func nearestRFC3164Time(month, day, clock string, received time.Time, loc *time.Location) (time.Time, error) {
	if loc == nil {
		loc = time.UTC
	}
	base := received.In(loc)
	value := month + " " + day + " " + clock
	best := time.Time{}
	bestDelta := time.Duration(1<<63 - 1)
	for _, year := range []int{base.Year() - 1, base.Year(), base.Year() + 1} {
		t, err := time.ParseInLocation("2006 Jan 2 15:04:05", strconv.Itoa(year)+" "+value, loc)
		if err != nil {
			return time.Time{}, err
		}
		delta := t.Sub(base)
		if delta < 0 {
			delta = -delta
		}
		if delta < bestDelta {
			best, bestDelta = t, delta
		}
	}
	return best, nil
}

func parseTag(value string) (app, pid, message string) {
	value = strings.TrimSpace(value)
	head, rest, found := strings.Cut(value, ":")
	if !found {
		return "", "", value
	}
	message = strings.TrimSpace(rest)
	app = head
	if open := strings.LastIndexByte(head, '['); open > 0 && strings.HasSuffix(head, "]") {
		app, pid = head[:open], head[open+1:len(head)-1]
	}
	return app, pid, message
}

type RFC5424 struct{}

func (RFC5424) Name() string { return "rfc5424" }

func (RFC5424) Parse(_ context.Context, payload []byte, received time.Time, _ *time.Location) (Event, error) {
	s := strings.TrimRight(string(payload), "\r\n")
	if !strings.HasPrefix(s, "<") {
		return Event{}, fmt.Errorf("%w: priority", ErrInvalid)
	}
	end := strings.IndexByte(s, '>')
	if end < 2 || end > 4 {
		return Event{}, fmt.Errorf("%w: priority", ErrInvalid)
	}
	p, facility, severity, err := priority(s[1:end])
	if err != nil {
		return Event{}, err
	}
	rest := s[end+1:]
	parts := make([]string, 0, 6)
	for i := 0; i < 6; i++ {
		token, remaining, ok := cutToken(rest)
		if !ok {
			return Event{}, fmt.Errorf("%w: RFC5424 header", ErrInvalid)
		}
		parts = append(parts, token)
		rest = remaining
	}
	if parts[0] != "1" {
		return Event{}, fmt.Errorf("%w: unsupported RFC5424 version", ErrInvalid)
	}
	var stamp time.Time
	timestampInferred := false
	if parts[1] == "-" {
		stamp = received
		timestampInferred = true
	} else {
		stamp, err = time.Parse(time.RFC3339Nano, parts[1])
		if err != nil {
			return Event{}, fmt.Errorf("%w: timestamp", ErrInvalid)
		}
	}
	fields := map[string]any{}
	remaining, err := parseStructuredData(rest, fields)
	if err != nil {
		return Event{}, err
	}
	message := strings.TrimPrefix(remaining, " ")
	message = strings.TrimPrefix(message, "\uFEFF")
	return Event{Timestamp: stamp.UTC(), TimestampInferred: timestampInferred, Message: message, Hostname: nilValue(parts[2]), Facility: facility, Severity: severity, Priority: p, AppName: nilValue(parts[3]), ProcessID: nilValue(parts[4]), MessageID: nilValue(parts[5]), Format: "rfc5424", Fields: fields}, nil
}

func cutToken(value string) (string, string, bool) {
	i := strings.IndexByte(value, ' ')
	if i < 0 {
		return "", "", false
	}
	return value[:i], strings.TrimLeft(value[i+1:], " "), true
}

func nilValue(v string) string {
	if v == "-" {
		return ""
	}
	return v
}

func parseStructuredData(joined string, fields map[string]any) (string, error) {
	if joined == "-" {
		return "", nil
	}
	if strings.HasPrefix(joined, "- ") {
		return joined[2:], nil
	}
	if joined == "" || joined[0] != '[' {
		return "", fmt.Errorf("%w: structured data", ErrInvalid)
	}
	i := 0
	for i < len(joined) && joined[i] == '[' {
		i++
		start := i
		for i < len(joined) && joined[i] != ' ' && joined[i] != ']' {
			i++
		}
		if i == start {
			return "", fmt.Errorf("%w: structured data id", ErrInvalid)
		}
		sdID := joined[start:i]
		for i < len(joined) && joined[i] != ']' {
			for i < len(joined) && joined[i] == ' ' {
				i++
			}
			nameStart := i
			for i < len(joined) && joined[i] != '=' && joined[i] != ']' && joined[i] != ' ' {
				i++
			}
			if i == nameStart || i >= len(joined) || joined[i] != '=' {
				return "", fmt.Errorf("%w: structured data parameter", ErrInvalid)
			}
			name := joined[nameStart:i]
			i++
			if i >= len(joined) || joined[i] != '"' {
				return "", fmt.Errorf("%w: structured data value", ErrInvalid)
			}
			i++
			var value strings.Builder
			closed := false
			for i < len(joined) {
				ch := joined[i]
				i++
				if ch == '"' {
					closed = true
					break
				}
				if ch == '\\' {
					if i >= len(joined) {
						break
					}
					escaped := joined[i]
					i++
					if escaped == '"' || escaped == '\\' || escaped == ']' {
						ch = escaped
					} else {
						value.WriteByte('\\')
						ch = escaped
					}
				}
				value.WriteByte(ch)
			}
			if !closed {
				return "", fmt.Errorf("%w: unterminated structured data value", ErrInvalid)
			}
			fields["syslog.sd."+sdID+"."+name] = value.String()
		}
		if i >= len(joined) || joined[i] != ']' {
			return "", fmt.Errorf("%w: unterminated structured data", ErrInvalid)
		}
		i++
	}
	return joined[i:], nil
}
