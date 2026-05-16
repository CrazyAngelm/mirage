package singbox

import "encoding/json"

const geoipRuSRSURL = "https://raw.githubusercontent.com/runetfreedom/russia-v2ray-rules-dat/release/sing-box/rule-set-geoip/geoip-ru.srs"

func MixedProxyConfig(activeTag string, outbounds []map[string]any) (string, error) {
	return MixedProxyConfigWithRouting(activeTag, outbounds, nil, false, true)
}

func MixedProxyConfigWithEndpoints(activeTag string, outbounds []map[string]any, endpoints []map[string]any) (string, error) {
	return MixedProxyConfigWithRouting(activeTag, outbounds, endpoints, false, true)
}

func MixedProxyConfigURLTest(outbounds []map[string]any, endpoints []map[string]any) (string, error) {
	return MixedProxyConfigWithRouting("auto", outbounds, endpoints, true, true)
}

func MixedProxyConfigWithRouting(activeTag string, outbounds []map[string]any, endpoints []map[string]any, useURLTest bool, russianDirect bool) (string, error) {
	return buildConfig(activeTag, outbounds, endpoints, useURLTest, russianDirect)
}

func TunConfig(activeTag string, outbounds []map[string]any) (string, error) {
	return TunConfigWithRouting(activeTag, outbounds, nil, false, true)
}

func TunConfigWithEndpoints(activeTag string, outbounds []map[string]any, endpoints []map[string]any) (string, error) {
	return TunConfigWithRouting(activeTag, outbounds, endpoints, false, true)
}

func TunConfigURLTest(outbounds []map[string]any, endpoints []map[string]any) (string, error) {
	return TunConfigWithRouting("auto", outbounds, endpoints, true, true)
}

func TunConfigWithRouting(activeTag string, outbounds []map[string]any, endpoints []map[string]any, useURLTest bool, russianDirect bool) (string, error) {
	return buildTunConfig(activeTag, outbounds, endpoints, useURLTest, russianDirect)
}

func buildConfig(activeTag string, outbounds []map[string]any, endpoints []map[string]any, useURLTest bool, russianDirect bool) (string, error) {
	allOutbounds := make([]map[string]any, 0, len(outbounds)+3)
	allOutbounds = append(allOutbounds, outbounds...)
	if useURLTest {
		allOutbounds = append(allOutbounds, buildURLTestOutbound(outbounds, endpoints))
	}
	allOutbounds = appendCoreOutbounds(allOutbounds)
	ruleSetDetour := ruleSetDownloadDetour(activeTag, outbounds, endpoints)
	route := map[string]any{"auto_detect_interface": true, "final": activeTag}
	if russianDirect {
		route["rule_set"] = russianDirectRuleSets(ruleSetDetour)
		route["rules"] = []any{russianDomainDirectRule(), russianDirectRule()}
	}
	cfg := map[string]any{
		"log":       map[string]any{"level": "info"},
		"dns":       dnsConfig(activeTag, russianDirect, true, ruleSetDetour),
		"inbounds":  []any{map[string]any{"type": "mixed", "tag": "mixed-in", "listen": "127.0.0.1", "listen_port": 2080}},
		"outbounds": allOutbounds,
		"route":     route,
	}
	if len(endpoints) > 0 {
		cfg["endpoints"] = endpoints
	}
	payload, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	return string(payload) + "\n", nil
}

func buildTunConfig(activeTag string, outbounds []map[string]any, endpoints []map[string]any, useURLTest bool, russianDirect bool) (string, error) {
	allOutbounds := make([]map[string]any, 0, len(outbounds)+3)
	allOutbounds = append(allOutbounds, outbounds...)
	if useURLTest {
		allOutbounds = append(allOutbounds, buildURLTestOutbound(outbounds, endpoints))
	}
	allOutbounds = appendCoreOutbounds(allOutbounds)
	ruleSetDetour := ruleSetDownloadDetour(activeTag, outbounds, endpoints)
	rules := []any{
		map[string]any{"protocol": "dns", "action": "hijack-dns"},
		map[string]any{"ip_is_private": true, "outbound": "direct"},
	}
	route := map[string]any{
		"auto_detect_interface": true,
		"final":                 activeTag,
		"rules":                 rules,
	}
	if russianDirect {
		route["rule_set"] = russianDirectRuleSets(ruleSetDetour)
		route["rules"] = append(rules, russianDomainDirectRule(), russianDirectRule())
	}
	cfg := map[string]any{
		"log": map[string]any{"level": "info"},
		"dns": dnsConfig(activeTag, russianDirect, false, ruleSetDetour),
		"inbounds": []any{
			map[string]any{
				"type":           "tun",
				"tag":            "tun-in",
				"interface_name": "Mirage",
				"address":        []string{"172.19.0.1/30", "fdfe:dcba:9876::1/126"},
				"mtu":            9000,
				"auto_route":     true,
				"strict_route":   true,
				"stack":          "system",
			},
		},
		"outbounds": allOutbounds,
		"route":     route,
	}
	if len(endpoints) > 0 {
		cfg["endpoints"] = endpoints
	}
	payload, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	return string(payload) + "\n", nil
}

func dnsConfig(activeTag string, russianDirect bool, mixedProxy bool, russianDirectDetour string) map[string]any {
	final := "cloudflare"
	if russianDirect && mixedProxy {
		final = "ru-direct-dns"
	}
	cfg := map[string]any{
		"servers": []any{
			map[string]any{"tag": "cloudflare", "type": "udp", "server": "1.1.1.1", "detour": activeTag},
			map[string]any{"tag": "ru-direct-dns", "type": "tcp", "server": "1.1.1.1", "detour": russianDirectDetour},
		},
		"final": final,
	}
	if russianDirect {
		cfg["rules"] = []any{
			map[string]any{"domain_suffix": []any{".ru", ".рф"}, "action": "route", "server": "ru-direct-dns"},
		}
	}
	return cfg
}

func ruleSetDownloadDetour(activeTag string, outbounds []map[string]any, endpoints []map[string]any) string {
	for _, outbound := range outbounds {
		if outbound["tag"] == "xray_reality_xhttp" {
			return "xray_reality_xhttp"
		}
	}
	if activeTag != "auto" {
		return activeTag
	}
	for _, endpoint := range endpoints {
		if endpoint["tag"] == "xray_reality_xhttp" {
			return "xray_reality_xhttp"
		}
	}
	return activeTag
}

func appendCoreOutbounds(outbounds []map[string]any) []map[string]any {
	return append(outbounds,
		map[string]any{"type": "direct", "tag": "direct", "domain_resolver": "ru-direct-dns"},
		map[string]any{"type": "block", "tag": "block"},
	)
}

func russianDirectRuleSets(downloadDetour string) []any {
	return []any{
		map[string]any{
			"type":            "remote",
			"tag":             "geoip-ru",
			"format":          "binary",
			"url":             geoipRuSRSURL,
			"download_detour": downloadDetour,
		},
	}
}

func russianDomainDirectRule() map[string]any {
	return map[string]any{
		"domain_suffix": []any{"ru", "рф"},
		"outbound":      "direct",
	}
}

func russianDirectRule() map[string]any {
	return map[string]any{
		"rule_set": []any{"geoip-ru"},
		"outbound": "direct",
	}
}

func BuildWireGuardEndpoint(server string, port int, clientPrivateKey string, peerPublicKey string, address string) map[string]any {
	return map[string]any{
		"type":        "wireguard",
		"tag":         "amneziawg",
		"address":     []string{address},
		"private_key": clientPrivateKey,
		"peers": []any{
			map[string]any{
				"address":     server,
				"port":        port,
				"public_key":  peerPublicKey,
				"allowed_ips": []string{"0.0.0.0/0"},
			},
		},
	}
}

func buildURLTestOutbound(outbounds []map[string]any, endpoints []map[string]any) map[string]any {
	tags := make([]string, 0, len(outbounds)+len(endpoints))
	for _, o := range outbounds {
		if tag, ok := o["tag"].(string); ok && tag != "" {
			tags = append(tags, tag)
		}
	}
	for _, ep := range endpoints {
		if tag, ok := ep["tag"].(string); ok && tag != "" {
			tags = append(tags, tag)
		}
	}
	return map[string]any{
		"type":      "urltest",
		"tag":       "auto",
		"outbounds": tags,
		"url":       "http://www.gstatic.com/generate_204",
		"interval":  "30s",
	}
}
