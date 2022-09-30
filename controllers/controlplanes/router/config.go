package router

import (
	"strconv"
	"strings"
)

func GetConfig() string {
	replacer := strings.NewReplacer("<MESSAGE_PORT>", strconv.Itoa(MessagePort),
		"<HTTP_PORT>", strconv.Itoa(HTTPPort),
		"<INTERIOR_PORT>", strconv.Itoa(InteriorPort),
		"<EDGE_PORT>", strconv.Itoa(EdgePort))

	return replacer.Replace(rawRouterConfig)
}

const (
	MessagePort  = 5672
	HTTPPort     = 9090
	InteriorPort = 55672
	EdgePort     = 45672
)

const rawRouterConfig = `
router {
    mode: interior
    id: default-router
}

listener {
    host: 0.0.0.0
    port: <MESSAGE_PORT>
    role: normal
}

sslProfile {
    name: router-amqps
    certFile: /etc/qpid-dispatch-certs/router-amqps/tls.crt
    privateKeyFile: /etc/qpid-dispatch-certs/router-amqps/tls.key
    caCertFile: /etc/qpid-dispatch-certs/router-amqps/ca.crt
}

listener {
    host: 0.0.0.0
    port: <HTTP_PORT>
    role: normal
    http: true
    httpRootDir: disabled
    websockets: false
    healthz: true
    metrics: true
}

sslProfile {
    name: router-internal
    certFile: /etc/qpid-dispatch-certs/router-internal/tls.crt
    privateKeyFile: /etc/qpid-dispatch-certs/router-internal/tls.key
    caCertFile: /etc/qpid-dispatch-certs/router-internal/ca.crt
}

listener {
    role: inter-router
    host: 0.0.0.0
    port: <INTERIOR_PORT>
    saslMechanisms: ANONYMOUS
    authenticatePeer: no
}

listener {
    role: edge
    host: 0.0.0.0
    port: <EDGE_PORT>
    saslMechanisms: ANONYMOUS
    authenticatePeer: no
}

`
