/*
 *  *******************************************************************************
 *  * Copyright (c) 2023 Contributors to the Eclipse ioFog Project
 *  *
 *  * This program and the accompanying materials are made available under the
 *  * terms of the Eclipse Public License v. 2.0 which is available at
 *  * http://www.eclipse.org/legal/epl-2.0
 *  *
 *  * SPDX-License-Identifier: EPL-2.0
 *  *******************************************************************************
 *
 */

package nats

import (
	"fmt"
	"strconv"
	"strings"
)

// Default NATS ports (aligned with Controller nats-hub-service.js).
const (
	DefaultServerPort  = 4222
	DefaultClusterPort = 6222
	DefaultLeafPort    = 7422
	DefaultMqttPort    = 8883
	DefaultHttpPort    = 8222
)

// Default JetStream storage sizes for NATS server.conf (max_file_store, max_memory_store).
// NATS uses decimal units: G, M, T, K (not Gi, Mi, Ti, Ki).
const (
	DefaultStorageSize     = "10G"
	DefaultMemoryStoreSize = "1G"
)

// DefaultStorageSizePVC is the default PVC size for JetStream file store; Kubernetes uses Gi/Mi.
const DefaultStorageSizePVC = "10Gi"

// ConfigMap and Secret names (ControlPlane namespace).
const (
	ConfigMapName       = "iofog-nats-config"
	JWTBundleCMName     = "iofog-nats-jwt-bundle"
	HeadlessServiceName = "nats-headless"
	ClientServiceName   = "nats"
	ServerServiceName   = "nats-server"
)

// JetStream key secret name suffix: nats-jetstream-key-<controlplane-name>
func JetStreamKeySecretName(controlplaneName string) string {
	return "nats-jetstream-key-" + controlplaneName
}

// ToNatsSize converts a size string to NATS units (G, M, T, K). NATS does not support Gi, Mi, Ti, Ki.
// Use for max_file_store and max_memory_store in server.conf. Handles lower and upper case (e.g. 10gi, 10Gi -> 10G).
func ToNatsSize(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	lower := strings.ToLower(s)
	switch {
	case strings.HasSuffix(lower, "gi"):
		return s[:len(s)-2] + "G"
	case strings.HasSuffix(lower, "mi"):
		return s[:len(s)-2] + "M"
	case strings.HasSuffix(lower, "ti"):
		return s[:len(s)-2] + "T"
	case strings.HasSuffix(lower, "ki"):
		return s[:len(s)-2] + "K"
	}
	return s
}

// server.conf template. Placeholders: $NATS_SERVER_PORT$, $NATS_HTTP_PORT$,
// $OPERATOR_JWT$, $SYSTEM_ACCOUNT$, $JETSTREAM_DOMAIN$, $JETSTREAM_KEY$, $JETSTREAM_PREV_KEY$,
// $NATS_CLUSTER_ROUTES$, $NATS_SSL_DIR$, $NATS_CERT_NAME$, $NATS_MQTT_CERT_NAME$,
// $NATS_LEAF_PORT$, $NATS_LEAF_ADVERTISE$, $NATS_CLUSTER_PORT$, $NATS_MQTT_PORT$, $NATS_JWT_DIR$, $CONTROLLER_NAME$,
// $MAX_MEMORY_STORE$, $MAX_FILE_STORE$.
const serverConfTemplate = `port: $NATS_SERVER_PORT$
server_name: $SELFNAME
pid_file: /home/runner/run/nats.pid
http_port: $NATS_HTTP_PORT$

# Operator
operator = $OPERATOR_JWT$

# System account
system_account = $SYSTEM_ACCOUNT$

jetstream: {
  store_dir: /home/runner/data
  domain: $JETSTREAM_DOMAIN$
  max_memory_store: $MAX_MEMORY_STORE$
  max_file_store: $MAX_FILE_STORE$
  cipher: chachapoly
  key: "$JETSTREAM_KEY$"
  prev_encryption_key: "$JETSTREAM_PREV_KEY$"
}

cluster: {
  name: $CONTROLLER_NAME$
  port: $NATS_CLUSTER_PORT$
  no_advertise: true
  routes: $NATS_CLUSTER_ROUTES$
  tls: {
    ca_file: "$NATS_SSL_DIR$/$NATS_CERT_NAME$/ca.crt"
    cert_file: "$NATS_SSL_DIR$/$NATS_CERT_NAME$/tls.crt"
    key_file: "$NATS_SSL_DIR$/$NATS_CERT_NAME$/tls.key"
    handshake_first: true
    verify: true
    timeout: "3s"
  }
}

leafnodes: {
  port: $NATS_LEAF_PORT$
  advertise: $NATS_LEAF_ADVERTISE$
  tls: {
    ca_file: "$NATS_SSL_DIR$/$NATS_CERT_NAME$/ca.crt"
    cert_file: "$NATS_SSL_DIR$/$NATS_CERT_NAME$/tls.crt"
    key_file: "$NATS_SSL_DIR$/$NATS_CERT_NAME$/tls.key"
    verify: true
	handshake_first: true
    timeout: "3s"
  }
}

mqtt: {
  port: $NATS_MQTT_PORT$
  tls: {
    ca_file: "$NATS_SSL_DIR$/$NATS_MQTT_CERT_NAME$/ca.crt"
    cert_file: "$NATS_SSL_DIR$/$NATS_MQTT_CERT_NAME$/tls.crt"
    key_file: "$NATS_SSL_DIR$/$NATS_MQTT_CERT_NAME$/tls.key"
    handshake_first: true
    timeout: "3s"
  }
}

resolver: {
  type: full
  dir: "$NATS_JWT_DIR$"
  allow_delete: false
  interval: "2m"
}
`

// ServerConfParams holds values to fill into the server.conf template.
type ServerConfParams struct {
	ServerPort      int
	HttpPort        int
	OperatorJWT     string
	SystemAccount   string
	JetStreamDomain string
	JetStreamKey    string
	JetStreamPrev   string
	ClusterRoutes   string
	SSLDir          string
	CertName        string
	MqttCertName    string
	LeafPort        int
	LeafAdvertise   string
	ClusterPort     int
	MqttPort        int
	JWTDir          string
	ControllerName  string
	MaxMemoryStore  string
	MaxFileStore    string
}

// escapeConfString escapes double quotes in s for use inside a double-quoted NATS config value.
// Caller must wrap the result in quotes in the template (e.g. key: "$JETSTREAM_KEY$") so values
// that look like numbers (e.g. base64 "2E6...") or contain special characters are parsed as strings.
func escapeConfString(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
}

// BuildServerConf returns server.conf content with placeholders replaced.
// SELFNAME is left as $SELFNAME so the NATS image can substitute from the pod's metadata.name (downward API).
func BuildServerConf(p ServerConfParams) string {
	s := serverConfTemplate
	repl := []string{
		"$NATS_SERVER_PORT$", fmt.Sprintf("%d", p.ServerPort),
		"$NATS_HTTP_PORT$", fmt.Sprintf("%d", p.HttpPort),
		"$OPERATOR_JWT$", p.OperatorJWT,
		"$SYSTEM_ACCOUNT$", p.SystemAccount,
		"$JETSTREAM_DOMAIN$", p.JetStreamDomain,
		"$JETSTREAM_KEY$", escapeConfString(p.JetStreamKey),
		"$JETSTREAM_PREV_KEY$", escapeConfString(p.JetStreamPrev),
		"$NATS_CLUSTER_ROUTES$", p.ClusterRoutes,
		"$NATS_SSL_DIR$", p.SSLDir,
		"$NATS_CERT_NAME$", p.CertName,
		"$NATS_MQTT_CERT_NAME$", p.MqttCertName,
		"$NATS_LEAF_PORT$", fmt.Sprintf("%d", p.LeafPort),
		"$NATS_LEAF_ADVERTISE$", p.LeafAdvertise,
		"$NATS_CLUSTER_PORT$", fmt.Sprintf("%d", p.ClusterPort),
		"$NATS_MQTT_PORT$", fmt.Sprintf("%d", p.MqttPort),
		"$NATS_JWT_DIR$", p.JWTDir,
		"$CONTROLLER_NAME$", p.ControllerName,
		"$MAX_MEMORY_STORE$", p.MaxMemoryStore,
		"$MAX_FILE_STORE$", p.MaxFileStore,
	}
	return strings.NewReplacer(repl...).Replace(s)
}

// ClusterRoutesFormat returns the NATS cluster routes as a YAML array string for server.conf.
// NATS expects routes to be an array, not a comma-separated string (avoids "interface {} is string, not []interface {}").
func ClusterRoutesFormat(headlessName string, replicas int) string {
	parts := make([]string, replicas)
	for i := 0; i < replicas; i++ {
		parts[i] = fmt.Sprintf(`"nats://nats-%d.%s:%d"`, i, headlessName, DefaultClusterPort)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// parseRoutesFromServerConf extracts the cluster.routes array from existing server.conf content.
// Returns nil if the routes line cannot be found or parsed (caller should fall back to ClusterRoutesFormat).
// Each returned string is a raw URL (e.g. "nats://nats-0.nats-headless:6222").
// Splits on comma and trims each segment so multiline or comma-without-space formats parse correctly.
func parseRoutesFromServerConf(serverConf string) []string {
	idx := strings.Index(serverConf, "routes:")
	if idx < 0 {
		return nil
	}
	rest := serverConf[idx+len("routes:"):]
	start := strings.Index(rest, "[")
	if start < 0 {
		return nil
	}
	end := strings.Index(rest[start:], "]")
	if end < 0 {
		return nil
	}
	inner := strings.TrimSpace(rest[start+1 : start+end])
	if inner == "" {
		return []string{}
	}
	var raw []string
	for _, s := range strings.Split(inner, ",") {
		s = strings.TrimSpace(s)
		s = strings.Trim(s, `"`)
		s = strings.TrimRight(s, ",")
		s = strings.TrimSpace(s)
		if s != "" {
			raw = append(raw, s)
		}
	}
	return raw
}

// isK8sOrdinalRoute returns true if the route URL is a K8s StatefulSet ordinal route we generate.
func isK8sOrdinalRoute(route, headlessName string, clusterPort int) bool {
	prefix := "nats://nats-"
	suffix := fmt.Sprintf(".%s:%d", headlessName, clusterPort)
	if !strings.HasPrefix(route, prefix) || !strings.HasSuffix(route, suffix) {
		return false
	}
	mid := strings.TrimPrefix(strings.TrimSuffix(route, suffix), prefix)
	_, err := strconv.Atoi(mid)
	return err == nil
}

// ClusterRoutesMerge returns cluster routes for server.conf: K8s ordinal routes for 0..replicas-1
// plus any existing non-K8s routes (e.g. controller-added agent nodes). If existingServerConf is
// empty or parsing fails, returns ClusterRoutesFormat(headlessName, replicas).
// Operator-managed routes are replaced (not appended); other routes are deduplicated.
func ClusterRoutesMerge(headlessName string, replicas int, clusterPort int, existingServerConf string) string {
	if existingServerConf == "" {
		return ClusterRoutesFormat(headlessName, replicas)
	}
	existing := parseRoutesFromServerConf(existingServerConf)
	if existing == nil {
		return ClusterRoutesFormat(headlessName, replicas)
	}
	// Build desired K8s routes (replaces any existing K8s ordinal routes)
	k8sRoutes := make([]string, 0, replicas)
	for i := 0; i < replicas; i++ {
		k8sRoutes = append(k8sRoutes, fmt.Sprintf(`"nats://nats-%d.%s:%d"`, i, headlessName, clusterPort))
	}
	k8sSet := make(map[string]struct{}, len(k8sRoutes))
	for _, q := range k8sRoutes {
		k8sSet[strings.Trim(q, `"`)] = struct{}{}
	}
	// Collect non-K8s routes, normalizing so misparsed K8s routes are still classified correctly
	var otherRoutes []string
	seenOther := make(map[string]struct{})
	for _, r := range existing {
		norm := strings.TrimSpace(strings.TrimRight(strings.Trim(r, `"`), ","))
		if norm == "" {
			continue
		}
		if isK8sOrdinalRoute(norm, headlessName, clusterPort) {
			continue
		}
		if _, ok := k8sSet[norm]; ok {
			continue
		}
		if _, ok := seenOther[norm]; ok {
			continue
		}
		seenOther[norm] = struct{}{}
		otherRoutes = append(otherRoutes, norm)
	}
	for _, r := range otherRoutes {
		k8sRoutes = append(k8sRoutes, `"`+r+`"`)
	}
	return "[" + strings.Join(k8sRoutes, ", ") + "]"
}
