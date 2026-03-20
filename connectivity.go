package main

import (
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"
)

const (
	defaultForwardingProbeTimeout = 1500 * time.Millisecond
	maxForwardingProbeTimeout     = 5 * time.Second
)

func testForwardingTarget(dstIP string, dstPort int, protocol string, timeout time.Duration) ([]ForwardingProbeResult, string, error) {
	protocols, err := expandForwardingProbeProtocols(protocol)
	if err != nil {
		return nil, "", err
	}

	address := net.JoinHostPort(dstIP, fmt.Sprintf("%d", dstPort))
	results := make([]ForwardingProbeResult, 0, len(protocols))

	for _, proto := range protocols {
		switch proto {
		case "tcp":
			results = append(results, probeTCP(address, timeout))
		case "udp":
			results = append(results, probeUDP(address, timeout))
		}
	}

	return results, summarizeForwardingProbeStatus(results), nil
}

func expandForwardingProbeProtocols(protocol string) ([]string, error) {
	switch protocol {
	case "tcp":
		return []string{"tcp"}, nil
	case "udp":
		return []string{"udp"}, nil
	case "both":
		return []string{"tcp", "udp"}, nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}
}

func probeTCP(address string, timeout time.Duration) ForwardingProbeResult {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", address, timeout)
	duration := time.Since(start).Milliseconds()
	if err != nil {
		return ForwardingProbeResult{
			Protocol:   "tcp",
			Status:     "unreachable",
			Reachable:  false,
			Message:    formatForwardingProbeError("tcp", err),
			DurationMs: duration,
		}
	}
	_ = conn.Close()

	return ForwardingProbeResult{
		Protocol:   "tcp",
		Status:     "reachable",
		Reachable:  true,
		Message:    fmt.Sprintf("TCP handshake succeeded in %d ms", duration),
		DurationMs: duration,
	}
}

func probeUDP(address string, timeout time.Duration) ForwardingProbeResult {
	start := time.Now()
	conn, err := net.DialTimeout("udp", address, timeout)
	if err != nil {
		return ForwardingProbeResult{
			Protocol:   "udp",
			Status:     "unreachable",
			Reachable:  false,
			Message:    formatForwardingProbeError("udp", err),
			DurationMs: time.Since(start).Milliseconds(),
		}
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write([]byte{0}); err != nil {
		return ForwardingProbeResult{
			Protocol:   "udp",
			Status:     "unreachable",
			Reachable:  false,
			Message:    formatForwardingProbeError("udp", err),
			DurationMs: time.Since(start).Milliseconds(),
		}
	}

	buf := make([]byte, 1)
	_, err = conn.Read(buf)
	duration := time.Since(start).Milliseconds()
	if err == nil {
		return ForwardingProbeResult{
			Protocol:   "udp",
			Status:     "reachable",
			Reachable:  true,
			Message:    fmt.Sprintf("UDP peer replied in %d ms", duration),
			DurationMs: duration,
		}
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return ForwardingProbeResult{
			Protocol:   "udp",
			Status:     "unknown",
			Reachable:  false,
			Message:    fmt.Sprintf("UDP probe sent, but no reply or ICMP rejection arrived within %s", timeout.Round(100*time.Millisecond)),
			DurationMs: duration,
		}
	}

	status := "unknown"
	if isForwardingProbeUnreachable(err) {
		status = "unreachable"
	}

	return ForwardingProbeResult{
		Protocol:   "udp",
		Status:     status,
		Reachable:  false,
		Message:    formatForwardingProbeError("udp", err),
		DurationMs: duration,
	}
}

func summarizeForwardingProbeStatus(results []ForwardingProbeResult) string {
	if len(results) == 0 {
		return "unknown"
	}

	hasReachable := false
	hasUnknown := false
	hasUnreachable := false

	for _, result := range results {
		switch result.Status {
		case "reachable":
			hasReachable = true
		case "unreachable":
			hasUnreachable = true
		default:
			hasUnknown = true
		}
	}

	switch {
	case hasReachable && !hasUnknown && !hasUnreachable:
		return "reachable"
	case hasUnreachable && !hasReachable && !hasUnknown:
		return "unreachable"
	case hasUnknown && !hasReachable && !hasUnreachable:
		return "unknown"
	default:
		return "partial"
	}
}

func formatForwardingProbeError(protocol string, err error) string {
	switch {
	case errors.Is(err, syscall.ECONNREFUSED):
		return fmt.Sprintf("%s probe was rejected by the destination", protocolDisplayName(protocol))
	case errors.Is(err, syscall.EHOSTUNREACH):
		return fmt.Sprintf("%s destination host is unreachable", protocolDisplayName(protocol))
	case errors.Is(err, syscall.ENETUNREACH):
		return fmt.Sprintf("%s network is unreachable", protocolDisplayName(protocol))
	case errors.Is(err, syscall.ETIMEDOUT):
		return fmt.Sprintf("%s probe timed out", protocolDisplayName(protocol))
	default:
		return fmt.Sprintf("%s probe failed: %v", protocolDisplayName(protocol), err)
	}
}

func isForwardingProbeUnreachable(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.EHOSTUNREACH) ||
		errors.Is(err, syscall.ENETUNREACH) ||
		errors.Is(err, syscall.ETIMEDOUT)
}

func protocolDisplayName(protocol string) string {
	switch protocol {
	case "tcp":
		return "TCP"
	case "udp":
		return "UDP"
	default:
		return protocol
	}
}
