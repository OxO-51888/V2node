package core

import (
	"encoding/json"
	"net"
	"strings"

	panel "github.com/wyx2685/v2node/api/v2board"
	"github.com/wyx2685/v2node/conf"
	"github.com/xtls/xray-core/app/dns"
	"github.com/xtls/xray-core/app/router"
	xnet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/core"
	coreConf "github.com/xtls/xray-core/infra/conf"
)

// hasPublicIPv6 checks if the machine has a public IPv6 address
func hasPublicIPv6() bool {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		ip := ipNet.IP
		// Check if it's IPv6, not loopback, not link-local, not private/ULA
		if ip.To4() == nil && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsPrivate() {
			return true
		}
	}
	return false
}

func hasOutboundWithTag(list []*core.OutboundHandlerConfig, tag string) bool {
	for _, o := range list {
		if o != nil && o.Tag == tag {
			return true
		}
	}
	return false
}

func GetCustomConfig(infos []*panel.NodeInfo, unlock conf.UnlockConfig) (*dns.Config, []*core.OutboundHandlerConfig, *router.Config, error) {
	//dns
	queryStrategy := "UseIPv4v6"
	if !hasPublicIPv6() {
		queryStrategy = "UseIPv4"
	}
	coreDnsConfig := &coreConf.DNSConfig{
		Servers: []*coreConf.NameServerConfig{
			{
				Address: &coreConf.Address{
					Address: xnet.ParseAddress("localhost"),
				},
			},
		},
		QueryStrategy: queryStrategy,
	}
	//outbound
	defaultoutbound, _ := buildDefaultOutbound()
	coreOutboundConfig := append([]*core.OutboundHandlerConfig{}, defaultoutbound)
	block, _ := buildBlockOutbound()
	coreOutboundConfig = append(coreOutboundConfig, block)
	dns, _ := buildDnsOutbound()
	coreOutboundConfig = append(coreOutboundConfig, dns)

	//route
	domainStrategy := "AsIs"
	dnsRule, _ := json.Marshal(map[string]interface{}{
		"port":        "53",
		"network":     "udp",
		"outboundTag": "dns_out",
	})
	bittorrentRule, _ := json.Marshal(map[string]interface{}{
		"protocol":    []string{"bittorrent"},
		"outboundTag": "block",
	})
	coreRouterConfig := &coreConf.RouterConfig{
		RuleList:       []json.RawMessage{dnsRule, bittorrentRule},
		DomainStrategy: &domainStrategy,
	}

	for _, info := range infos {
		if len(info.Common.Routes) == 0 {
			continue
		}
		for _, route := range info.Common.Routes {
			switch route.Action {
			case "dns":
				if route.ActionValue == nil {
					continue
				}
				server := &coreConf.NameServerConfig{
					Address: &coreConf.Address{
						Address: xnet.ParseAddress(*route.ActionValue),
					},
				}
				if len(route.Match) != 0 {
					server.Domains = route.Match
					server.SkipFallback = true
				}
				coreDnsConfig.Servers = append(coreDnsConfig.Servers, server)
			case "block":
				rule := map[string]interface{}{
					"inboundTag":  info.Tag,
					"domain":      route.Match,
					"outboundTag": "block",
				}
				rawRule, err := json.Marshal(rule)
				if err != nil {
					continue
				}
				coreRouterConfig.RuleList = append(coreRouterConfig.RuleList, rawRule)
			case "block_ip":
				rule := map[string]interface{}{
					"inboundTag":  info.Tag,
					"ip":          route.Match,
					"outboundTag": "block",
				}
				rawRule, err := json.Marshal(rule)
				if err != nil {
					continue
				}
				coreRouterConfig.RuleList = append(coreRouterConfig.RuleList, rawRule)
			case "block_port":
				rule := map[string]interface{}{
					"inboundTag":  info.Tag,
					"port":        strings.Join(route.Match, ","),
					"outboundTag": "block",
				}
				rawRule, err := json.Marshal(rule)
				if err != nil {
					continue
				}
				coreRouterConfig.RuleList = append(coreRouterConfig.RuleList, rawRule)
			case "protocol":
				rule := map[string]interface{}{
					"inboundTag":  info.Tag,
					"protocol":    route.Match,
					"outboundTag": "block",
				}
				rawRule, err := json.Marshal(rule)
				if err != nil {
					continue
				}
				coreRouterConfig.RuleList = append(coreRouterConfig.RuleList, rawRule)
			case "route":
				if route.ActionValue == nil {
					continue
				}
				outbound := &coreConf.OutboundDetourConfig{}
				err := json.Unmarshal([]byte(*route.ActionValue), outbound)
				if err != nil {
					continue
				}
				rule := map[string]interface{}{
					"inboundTag":  info.Tag,
					"domain":      route.Match,
					"outboundTag": outbound.Tag,
				}
				rawRule, err := json.Marshal(rule)
				if err != nil {
					continue
				}
				coreRouterConfig.RuleList = append(coreRouterConfig.RuleList, rawRule)
				if hasOutboundWithTag(coreOutboundConfig, outbound.Tag) {
					continue
				}
				custom_outbound, err := outbound.Build()
				if err != nil {
					continue
				}
				coreOutboundConfig = append(coreOutboundConfig, custom_outbound)
			case "route_ip":
				if route.ActionValue == nil {
					continue
				}
				outbound := &coreConf.OutboundDetourConfig{}
				err := json.Unmarshal([]byte(*route.ActionValue), outbound)
				if err != nil {
					continue
				}
				rule := map[string]interface{}{
					"inboundTag":  info.Tag,
					"ip":          route.Match,
					"outboundTag": outbound.Tag,
				}
				rawRule, err := json.Marshal(rule)
				if err != nil {
					continue
				}
				coreRouterConfig.RuleList = append(coreRouterConfig.RuleList, rawRule)
				if hasOutboundWithTag(coreOutboundConfig, outbound.Tag) {
					continue
				}
				custom_outbound, err := outbound.Build()
				if err != nil {
					continue
				}
				coreOutboundConfig = append(coreOutboundConfig, custom_outbound)
			case "default_out":
				if route.ActionValue == nil {
					continue
				}
				outbound := &coreConf.OutboundDetourConfig{}
				err := json.Unmarshal([]byte(*route.ActionValue), outbound)
				if err != nil {
					continue
				}
				rule := map[string]interface{}{
					"inboundTag":  info.Tag,
					"network":     "tcp,udp",
					"outboundTag": outbound.Tag,
				}
				rawRule, err := json.Marshal(rule)
				if err != nil {
					continue
				}
				coreRouterConfig.RuleList = append(coreRouterConfig.RuleList, rawRule)
				if hasOutboundWithTag(coreOutboundConfig, outbound.Tag) {
					continue
				}
				custom_outbound, err := outbound.Build()
				if err != nil {
					continue
				}
				coreOutboundConfig = append(coreOutboundConfig, custom_outbound)
			default:
				continue
			}
		}
	}
	appendUnlockRoutes(&coreOutboundConfig, coreRouterConfig, unlock)
	DnsConfig, err := coreDnsConfig.Build()
	if err != nil {
		return nil, nil, nil, err
	}
	RouterConfig, err := coreRouterConfig.Build()
	if err != nil {
		return nil, nil, nil, err
	}
	return DnsConfig, coreOutboundConfig, RouterConfig, nil
}

func appendUnlockRoutes(outbounds *[]*core.OutboundHandlerConfig, router *coreConf.RouterConfig, unlock conf.UnlockConfig) {
	if !unlock.Enable {
		return
	}

	known := make(map[string]bool)
	for _, outbound := range *outbounds {
		if outbound != nil && outbound.Tag != "" {
			known[outbound.Tag] = true
		}
	}

	for _, socks := range unlock.SOCKS {
		if socks.Tag == "" || socks.Address == "" || socks.Port <= 0 || known[socks.Tag] {
			continue
		}
		outbound := buildUnlockSocksOutbound(socks)
		if outbound == nil {
			continue
		}
		*outbounds = append(*outbounds, outbound)
		known[socks.Tag] = true
	}

	for _, rule := range unlock.Rules {
		if rule.Outbound == "" || len(rule.Match) == 0 || !known[rule.Outbound] {
			continue
		}
		rawRule, err := json.Marshal(buildUnlockRulePayload(rule))
		if err != nil {
			continue
		}
		router.RuleList = append(router.RuleList, rawRule)
	}

	defaultOutbound := selectDefaultUnlockOutbound(unlock, known)
	if defaultOutbound != "" && !hasTwitterUnlockRule(unlock.Rules) {
		rawRule, err := json.Marshal(buildUnlockRulePayload(conf.UnlockRule{
			Outbound:  defaultOutbound,
			Match:     twitterUnlockDomains,
			ProtoPort: "tcp/443",
		}))
		if err == nil {
			router.RuleList = append(router.RuleList, rawRule)
		}
	}
}

var twitterUnlockDomains = []string{
	"domain:x.com",
	"domain:twitter.com",
	"domain:t.co",
	"domain:twimg.com",
	"domain:api.x.com",
	"domain:api.twitter.com",
	"domain:abs.twimg.com",
	"domain:pbs.twimg.com",
	"domain:video.twimg.com",
}

func selectDefaultUnlockOutbound(unlock conf.UnlockConfig, known map[string]bool) string {
	if tag := strings.TrimSpace(unlock.DefaultOutbound); tag != "" && known[tag] {
		return tag
	}
	for _, socks := range unlock.SOCKS {
		if socks.Tag != "" && known[socks.Tag] {
			return socks.Tag
		}
	}
	return ""
}

func hasTwitterUnlockRule(rules []conf.UnlockRule) bool {
	for _, rule := range rules {
		for _, match := range rule.Match {
			domain := unlockRuleDomain(match)
			if domain == "x.com" ||
				domain == "t.co" ||
				domain == "twitter.com" ||
				strings.HasSuffix(domain, ".twitter.com") ||
				domain == "twimg.com" ||
				strings.HasSuffix(domain, ".twimg.com") {
				return true
			}
		}
	}
	return false
}

func unlockRuleDomain(match string) string {
	domain := strings.ToLower(strings.TrimSpace(match))
	if index := strings.Index(domain, ":"); index >= 0 {
		domain = domain[index+1:]
	}
	return strings.Trim(domain, ".")
}

func buildUnlockRulePayload(rule conf.UnlockRule) map[string]interface{} {
	payload := map[string]interface{}{
		"domain":      rule.Match,
		"outboundTag": rule.Outbound,
	}
	protoPort := strings.TrimSpace(rule.ProtoPort)
	if protoPort == "" {
		return payload
	}
	parts := strings.SplitN(protoPort, "/", 2)
	if len(parts) == 2 {
		if network := strings.TrimSpace(parts[0]); network != "" {
			payload["network"] = network
		}
		if port := strings.TrimSpace(parts[1]); port != "" {
			payload["port"] = port
		}
		return payload
	}
	if protoPort == "tcp" || protoPort == "udp" || protoPort == "tcp,udp" {
		payload["network"] = protoPort
		return payload
	}
	payload["port"] = protoPort
	return payload
}

func buildUnlockSocksOutbound(socks conf.UnlockSOCKS) *core.OutboundHandlerConfig {
	server := map[string]interface{}{
		"address": socks.Address,
		"port":    socks.Port,
	}
	if socks.Username != "" && socks.Password != "" {
		server["users"] = []map[string]string{{
			"user": socks.Username,
			"pass": socks.Password,
		}}
	}

	outboundDetourConfig := &coreConf.OutboundDetourConfig{
		Protocol: "socks",
		Tag:      socks.Tag,
	}
	setting, err := json.Marshal(map[string]interface{}{
		"servers": []map[string]interface{}{server},
	})
	if err != nil {
		return nil
	}
	raw := json.RawMessage(setting)
	outboundDetourConfig.Settings = &raw
	built, err := outboundDetourConfig.Build()
	if err != nil {
		return nil
	}
	return built
}
