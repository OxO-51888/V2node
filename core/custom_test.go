package core

import (
	"strings"
	"testing"

	"github.com/wyx2685/v2node/conf"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestCustomConfigBlocksBittorrentByDefault(t *testing.T) {
	_, _, routerConfig, err := GetCustomConfig(nil, conf.UnlockConfig{})
	if err != nil {
		t.Fatalf("GetCustomConfig() error = %v", err)
	}

	raw, err := protojson.Marshal(routerConfig)
	if err != nil {
		t.Fatalf("marshal router config: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "bittorrent") {
		t.Fatalf("router config does not contain bittorrent block rule: %s", text)
	}
	if !strings.Contains(text, "block") {
		t.Fatalf("router config does not contain block outbound rule: %s", text)
	}
}

func TestCustomConfigAddsUnlockSocksRoutes(t *testing.T) {
	_, outbounds, routerConfig, err := GetCustomConfig(nil, conf.UnlockConfig{
		Enable: true,
		SOCKS: []conf.UnlockSOCKS{{
			Tag:     "sg",
			Address: "127.0.0.1",
			Port:    22220,
		}},
		Rules: []conf.UnlockRule{{
			Outbound:  "sg",
			Match:     []string{"domain:netflix.com"},
			ProtoPort: "tcp/443",
		}},
	})
	if err != nil {
		t.Fatalf("GetCustomConfig() error = %v", err)
	}

	hasSG := false
	for _, outbound := range outbounds {
		if outbound != nil && outbound.Tag == "sg" {
			hasSG = true
			break
		}
	}
	if !hasSG {
		t.Fatalf("unlock socks outbound was not added")
	}

	raw, err := protojson.Marshal(routerConfig)
	if err != nil {
		t.Fatalf("marshal router config: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "netflix.com") || !strings.Contains(text, "sg") || !strings.Contains(text, "443") {
		t.Fatalf("router config does not contain unlock route: %s", text)
	}
	if !strings.Contains(text, "x.com") || !strings.Contains(text, "video.twimg.com") {
		t.Fatalf("router config does not contain builtin twitter unlock route: %s", text)
	}
}
