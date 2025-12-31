// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
)

type Client interface {
	send(ctx context.Context, b batch) error
}

const sandboxLocalDomain = "sandbox.localdomain"

func NewClient(dst SubscriptionDestination) (Client, error) {
	switch dst.Protocol {
	case protocolTCP:
		return newTCPClient(dst.Port)
	case protocolHTTP:
		return newHTTPClient(dst.URI)
	default:
		return nil, fmt.Errorf("unknown protocol: %s. Only tcp and http are supported", dst.Protocol)
	}
}

type tcpClient struct {
	conn *bufio.Writer
}

func newTCPClient(port uint16) (*tcpClient, error) {
	address := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("could not TCP dial provided address %s: %w", address, err)
	}
	return &tcpClient{conn: bufio.NewWriter(conn)}, nil
}

func (c *tcpClient) send(ctx context.Context, b batch) error {
	for _, ev := range b.events {
		select {
		case <-ctx.Done():
			return fmt.Errorf("sending event to TCP subscriber was interrupted: %w", ctx.Err())
		default:
		}
		if _, err := c.conn.Write(ev); err != nil {
			return fmt.Errorf("could not write event: %w", err)
		}
		if _, err := c.conn.Write([]byte("\n")); err != nil {
			return fmt.Errorf("could not write event: %w", err)
		}
	}
	if err := c.conn.Flush(); err != nil {
		return fmt.Errorf("could not write events: %w", err)
	}
	return nil
}

type httpClient struct {
	addr string
}

func newHTTPClient(uri string) (*httpClient, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("could not parse destination.URI: %w", err)
	}
	if u.Hostname() != sandboxLocalDomain && u.Hostname() != "sandbox" {
		return nil, fmt.Errorf("destination.URI host must be %s", sandboxLocalDomain)
	}
	return &httpClient{addr: "http://127.0.0.1:" + u.Port()}, nil
}

func (c *httpClient) send(ctx context.Context, batch batch) error {
	b, err := json.Marshal(batch.events)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.addr, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("could not create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Sequence-Id", sequenceId(b))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("could not close response body", "err", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("could not read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("http request failed with status %s: %s", resp.Status, string(body))
	}

	slog.Debug("telemetry HTTP request completed", "extension_response", string(body))
	return nil
}

func sequenceId(data []byte) string {
	hash := sha256.Sum256(data)
	return base64.StdEncoding.EncodeToString(hash[:])
}
