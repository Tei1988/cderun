package config

import (
	"testing"
)

func BenchmarkResolveWithFS_FastPathStandard(b *testing.B) {
	cli := CLIOptions{
		Image:       ptr("alpine:latest"),
		TTY:         ptr(true),
		Interactive: ptr(false),
		Network:     ptr("bridge"),
		LogLevel:    ptr("error"),
		LogFormat:   ptr("text"),
		User:        ptr("1000:1000"),
		Workdir:     ptr("/app"),
	}
	mfs := &MockFileSystem{
		WD: "/app",
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ResolveWithFS("node", &cli, nil, nil, mfs)
		if err != nil {
			b.Fatalf("ResolveWithFS failed: %v", err)
		}
	}
}

func BenchmarkResolveWithFS_FastPathFullOptions(b *testing.B) {
	cli := CLIOptions{
		Image:          ptr("alpine:latest"),
		TTY:            ptr(true),
		Interactive:    ptr(true),
		ReadOnly:       ptr(true),
		Init:           ptr(true),
		Remove:         ptr(true),
		Privileged:     ptr(false),
		PublishAll:     ptr(false),
		LogTimestamp:   ptr(true),
		DryRun:         ptr(false),
		DryRunFormat:   ptr("yaml"),
		Network:        ptr("bridge"),
		LogLevel:       ptr("error"),
		LogFormat:      ptr("text"),
		User:           ptr("1000:1000"),
		Workdir:        ptr("/app"),
		Hostname:       ptr("test-host"),
		Pull:           ptr("missing"),
		PullMaxRetries: ptr(3),
		CPUShares:      ptr(1024),
		CPUs:           ptr(2.0),
		PidsLimit:      ptr(100),
		Ports:          []string{"8080:80"},
		Expose:         []string{"8080"},
		DNS:            []string{"8.8.8.8"},
		AddHosts:       []string{"host:127.0.0.1"},
		CapAdd:         []string{"SYS_PTRACE"},
		CapDrop:        []string{"ALL"},
		SecurityOpt:    []string{"no-new-privileges:true"},
		DNSSearch:      []string{"example.com"},
		DNSOptions:     []string{"ndots:2"},
	}
	mfs := &MockFileSystem{
		WD: "/app",
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ResolveWithFS("node", &cli, nil, nil, mfs)
		if err != nil {
			b.Fatalf("ResolveWithFS failed: %v", err)
		}
	}
}
