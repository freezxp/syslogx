package syslog

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/freezxp/syslogx/backend/internal/config"
	"github.com/freezxp/syslogx/backend/internal/domain"
	"github.com/freezxp/syslogx/backend/internal/ingestion"
	"github.com/freezxp/syslogx/backend/internal/metrics"
)

type Manager struct {
	sources  []config.SourceConfig
	pipeline *ingestion.Pipeline
	metrics  *metrics.Metrics
	logger   *slog.Logger
	mu       sync.Mutex
	closers  []io.Closer
	conns    map[net.Conn]struct{}
	acceptWG sync.WaitGroup
	connWG   sync.WaitGroup
}

func NewManager(sources []config.SourceConfig, pipeline *ingestion.Pipeline, m *metrics.Metrics, logger *slog.Logger) *Manager {
	return &Manager{sources: sources, pipeline: pipeline, metrics: m, logger: logger, conns: make(map[net.Conn]struct{})}
}

func (m *Manager) Start(ctx context.Context) error {
	for i := range m.sources {
		s := m.sources[i]
		if !s.Enabled {
			continue
		}
		var err error
		switch s.Protocol {
		case "udp":
			err = m.startUDP(ctx, s)
		case "tcp":
			err = m.startTCP(ctx, s)
		default:
			err = fmt.Errorf("unsupported protocol %q", s.Protocol)
		}
		if err != nil {
			m.Close()
			return fmt.Errorf("start source %q: %w", s.ID, err)
		}
	}
	return nil
}

func (m *Manager) addCloser(c io.Closer) {
	m.mu.Lock()
	m.closers = append(m.closers, c)
	m.mu.Unlock()
}

func (m *Manager) Close() error {
	m.mu.Lock()
	closers := append([]io.Closer(nil), m.closers...)
	m.closers = nil
	m.mu.Unlock()
	var result error
	for _, c := range closers {
		result = errors.Join(result, c.Close())
	}
	m.acceptWG.Wait()
	m.mu.Lock()
	connections := make([]net.Conn, 0, len(m.conns))
	for conn := range m.conns {
		connections = append(connections, conn)
	}
	m.mu.Unlock()
	for _, conn := range connections {
		result = errors.Join(result, conn.Close())
	}
	m.connWG.Wait()
	return result
}

func (m *Manager) startUDP(ctx context.Context, source config.SourceConfig) error {
	addr, err := net.ResolveUDPAddr("udp", source.Address)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	m.addCloser(conn)
	m.acceptWG.Add(1)
	go func() {
		defer m.acceptWG.Done()
		buffer := make([]byte, source.MaxMessageBytes+1)
		for {
			n, peer, err := conn.ReadFromUDP(buffer)
			if err != nil {
				if ctx.Err() == nil && !errors.Is(err, net.ErrClosed) {
					m.logger.Error("udp read failed", "source_id", source.ID, "error", err)
				}
				return
			}
			if n > source.MaxMessageBytes {
				m.pipeline.Reject("syslog_udp", "message_too_large", n)
				continue
			}
			payload := append([]byte(nil), buffer[:n]...)
			frame := makeFrame(source, "syslog_udp", payload, peer.IP.String(), peer.Port)
			if err := m.pipeline.Submit(ctx, frame); err != nil && !errors.Is(err, ingestion.ErrQueueFull) && !errors.Is(err, ingestion.ErrQueueClosed) {
				m.logger.Warn("udp admission failed", "source_id", source.ID, "error", err)
			}
		}
	}()
	m.logger.Info("syslog source started", "source_id", source.ID, "protocol", "udp", "address", conn.LocalAddr().String())
	return nil
}

func (m *Manager) startTCP(ctx context.Context, source config.SourceConfig) error {
	listener, err := net.Listen("tcp", source.Address)
	if err != nil {
		return err
	}
	m.addCloser(listener)
	sem := make(chan struct{}, source.MaxConnections)
	m.acceptWG.Add(1)
	go func() {
		defer m.acceptWG.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				if ctx.Err() == nil && !errors.Is(err, net.ErrClosed) {
					m.logger.Error("tcp accept failed", "source_id", source.ID, "error", err)
				}
				return
			}
			select {
			case sem <- struct{}{}:
				m.trackConnection(conn)
				m.connWG.Add(1)
				go func() {
					defer m.connWG.Done()
					defer func() { <-sem }()
					defer m.untrackConnection(conn)
					m.handleTCP(ctx, source, conn)
				}()
			default:
				_ = conn.Close()
				m.metrics.Dropped.WithLabelValues("admission", "connection_limit").Inc()
			}
		}
	}()
	m.logger.Info("syslog source started", "source_id", source.ID, "protocol", "tcp", "address", listener.Addr().String())
	return nil
}

func (m *Manager) trackConnection(conn net.Conn) {
	m.mu.Lock()
	m.conns[conn] = struct{}{}
	m.mu.Unlock()
}
func (m *Manager) untrackConnection(conn net.Conn) { m.mu.Lock(); delete(m.conns, conn); m.mu.Unlock() }

func (m *Manager) handleTCP(ctx context.Context, source config.SourceConfig, conn net.Conn) {
	defer conn.Close()
	m.metrics.ActiveConnections.WithLabelValues("syslog_tcp").Inc()
	defer m.metrics.ActiveConnections.WithLabelValues("syslog_tcp").Dec()
	peerIP, peerPort := peerAddress(conn.RemoteAddr())
	reader := bufio.NewReaderSize(conn, min(source.MaxMessageBytes+1, 256<<10))
	for {
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		payload, err := readFrame(reader, source.Framing, source.MaxMessageBytes)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return
			}
			if errors.Is(err, errFrameTooLarge) {
				m.pipeline.Reject("syslog_tcp", "message_too_large", 0)
			} else if ne, ok := err.(net.Error); !ok || !ne.Timeout() {
				m.logger.Debug("tcp frame ended", "source_id", source.ID, "error", err)
			}
			return
		}
		if len(payload) == 0 {
			continue
		}
		if err := m.pipeline.Submit(ctx, makeFrame(source, "syslog_tcp", payload, peerIP, peerPort)); err != nil {
			return
		}
	}
}

func makeFrame(source config.SourceConfig, protocol string, payload []byte, ip string, port int) domain.Frame {
	return domain.Frame{Payload: payload, ReceivedAt: time.Now().UTC(), TenantID: source.TenantID, SourceID: source.ID, SourceName: source.Name, Protocol: protocol, SourceIP: ip, SourcePort: uint16(port)}
}

func peerAddress(addr net.Addr) (string, int) {
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String(), 0
	}
	n, _ := strconv.Atoi(port)
	return host, n
}

var errFrameTooLarge = errors.New("syslog frame too large")

func readFrame(r *bufio.Reader, framing string, maxBytes int) ([]byte, error) {
	mode := framing
	if mode == "auto" {
		octet, err := looksOctetCounted(r)
		if err != nil {
			return nil, err
		}
		if octet {
			mode = "octet_counting"
		} else {
			mode = "newline"
		}
	}
	if mode == "octet_counting" {
		return readOctetCounted(r, maxBytes)
	}
	return readNewline(r, maxBytes)
}

func looksOctetCounted(r *bufio.Reader) (bool, error) {
	for i := 1; i <= 10; i++ {
		b, err := r.Peek(i)
		if err != nil {
			return false, err
		}
		ch := b[i-1]
		if ch == ' ' {
			return i > 1, nil
		}
		if ch < '0' || ch > '9' {
			return false, nil
		}
	}
	return false, errFrameTooLarge
}

func readOctetCounted(r *bufio.Reader, maxBytes int) ([]byte, error) {
	var digits strings.Builder
	for digits.Len() <= 10 {
		b, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		if b == ' ' {
			break
		}
		if b < '0' || b > '9' {
			return nil, fmt.Errorf("invalid octet count")
		}
		digits.WriteByte(b)
	}
	n, err := strconv.Atoi(digits.String())
	if err != nil || n < 0 {
		return nil, fmt.Errorf("invalid octet count")
	}
	if n > maxBytes {
		return nil, errFrameTooLarge
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func readNewline(r *bufio.Reader, maxBytes int) ([]byte, error) {
	var payload []byte
	for {
		part, err := r.ReadSlice('\n')
		payload = append(payload, part...)
		if len(payload) > maxBytes {
			return nil, errFrameTooLarge
		}
		if err == nil {
			return bytesTrimLine(payload), nil
		}
		if !errors.Is(err, bufio.ErrBufferFull) {
			if errors.Is(err, io.EOF) && len(payload) > 0 {
				return bytesTrimLine(payload), nil
			}
			return nil, err
		}
	}
}

func bytesTrimLine(b []byte) []byte {
	b = []byte(strings.TrimSuffix(string(b), "\n"))
	b = []byte(strings.TrimSuffix(string(b), "\r"))
	return b
}
