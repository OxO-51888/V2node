package core

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
)

func TestCustomConfigBlocksBittorrentByDefault(t *testing.T) {
	_, _, routerConfig, err := GetCustomConfig(nil)
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
