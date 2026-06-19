package nats

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildServerConf_OmitsLeafAdvertiseWhenEmpty(t *testing.T) {
	conf := BuildServerConf(ServerConfParams{
		ServerPort:     DefaultServerPort,
		HttpPort:       DefaultHttpPort,
		LeafPort:       DefaultLeafPort,
		LeafAdvertise:  "",
		ClusterPort:    DefaultClusterPort,
		MqttPort:       DefaultMqttPort,
		ClusterRoutes:  `["nats://nats-0.nats-headless:6222"]`,
		SSLDir:         "/etc/nats/certs",
		CertName:       "nats-site-server",
		MqttCertName:   "nats-mqtt-server",
		JWTDir:         "/home/runner/nats/jwt",
		ControllerName: "pot",
		MaxMemoryStore: "512M",
		MaxFileStore:   "2G",
	})

	require.NotContains(t, conf, "  advertise:")
	require.Contains(t, conf, "leafnodes: {")
	require.Contains(t, conf, "port: 7422")
	require.Contains(t, conf, "no_advertise: true")
}

func TestBuildServerConf_IncludesLeafAdvertiseWhenSet(t *testing.T) {
	conf := BuildServerConf(ServerConfParams{
		ServerPort:     DefaultServerPort,
		HttpPort:       DefaultHttpPort,
		LeafPort:       DefaultLeafPort,
		LeafAdvertise:  "nats.example.com:7422",
		ClusterPort:    DefaultClusterPort,
		MqttPort:       DefaultMqttPort,
		ClusterRoutes:  `["nats://nats-0.nats-headless:6222"]`,
		SSLDir:         "/etc/nats/certs",
		CertName:       "nats-site-server",
		MqttCertName:   "nats-mqtt-server",
		JWTDir:         "/home/runner/nats/jwt",
		ControllerName: "pot",
		MaxMemoryStore: "512M",
		MaxFileStore:   "2G",
	})

	require.Contains(t, conf, "  advertise: nats.example.com:7422")
	leafSection := conf[strings.Index(conf, "leafnodes: {"):]
	idxAdvertise := strings.Index(leafSection, "  advertise:")
	idxTLS := strings.Index(leafSection, "  tls: {")
	require.Greater(t, idxTLS, idxAdvertise)
}
