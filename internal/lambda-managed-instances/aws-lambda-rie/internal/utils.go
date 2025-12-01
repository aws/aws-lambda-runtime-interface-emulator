// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"

	"github.com/jessevdk/go-flags"
)

type Options struct {
	LogLevel       string `long:"log-level" description:"Log level (default: info). Can also be set via LOG_LEVEL env."`
	RuntimeAddress string `long:"runtime-api-address" description:"Address of the Lambda Runtime API."`
	RIEAddress     string `long:"runtime-interface-emulator-address" default:"0.0.0.0:8080" description:"Address for RIE to accept HTTP requests."`
}

func ParseCLIArgs(args []string) (Options, []string, error) {
	var opts Options
	parser := flags.NewParser(&opts, flags.IgnoreUnknown)
	args, err := parser.ParseArgs(args)

	if opts.LogLevel == "" {
		opts.LogLevel = os.Getenv("LOG_LEVEL")
		if opts.LogLevel == "" {
			opts.LogLevel = "info"
		}
	}

	return opts, args, err
}

func ConfigureLogging(levelStr string) {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(levelStr)); err != nil {
		lvl = slog.LevelInfo
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
	slog.SetDefault(logger)
}

func ParseAddr(addrStr, defaultAddr string) (netip.AddrPort, error) {
	if addrStr == "" {
		addrStr = defaultAddr
	}

	host, portStr, err := net.SplitHostPort(addrStr)
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("invalid address: %w", err)
	}

	port, err := net.LookupPort("tcp", portStr)
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("invalid port: %w", err)
	}

	ip, err := netip.ParseAddr(host)
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("invalid IP: %w", err)
	}

	return netip.AddrPortFrom(ip, uint16(port)), nil
}
