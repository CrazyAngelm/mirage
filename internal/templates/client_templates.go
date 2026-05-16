package templates

import "mirage/internal/singbox"

func SingBoxConfig(activeTag string, outbounds []map[string]any) (string, error) {
	return singbox.MixedProxyConfig(activeTag, outbounds)
}

func SingBoxConfigWithEndpoints(activeTag string, outbounds []map[string]any, endpoints []map[string]any) (string, error) {
	return singbox.MixedProxyConfigWithEndpoints(activeTag, outbounds, endpoints)
}

func SingBoxConfigURLTest(outbounds []map[string]any, endpoints []map[string]any) (string, error) {
	return singbox.MixedProxyConfigURLTest(outbounds, endpoints)
}

func SingBoxConfigWithRouting(activeTag string, outbounds []map[string]any, endpoints []map[string]any, useURLTest bool, russianDirect bool) (string, error) {
	return singbox.MixedProxyConfigWithRouting(activeTag, outbounds, endpoints, useURLTest, russianDirect)
}

func SingBoxTunConfig(activeTag string, outbounds []map[string]any) (string, error) {
	return singbox.TunConfig(activeTag, outbounds)
}

func SingBoxTunConfigWithEndpoints(activeTag string, outbounds []map[string]any, endpoints []map[string]any) (string, error) {
	return singbox.TunConfigWithEndpoints(activeTag, outbounds, endpoints)
}

func SingBoxTunConfigURLTest(outbounds []map[string]any, endpoints []map[string]any) (string, error) {
	return singbox.TunConfigURLTest(outbounds, endpoints)
}

func SingBoxTunConfigWithRouting(activeTag string, outbounds []map[string]any, endpoints []map[string]any, useURLTest bool, russianDirect bool) (string, error) {
	return singbox.TunConfigWithRouting(activeTag, outbounds, endpoints, useURLTest, russianDirect)
}
