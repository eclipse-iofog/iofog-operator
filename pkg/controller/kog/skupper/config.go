package skupper

import (
	"strconv"
	"strings"
)

func GetRouterConfig() string {
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
    id: Router.A
}

listener {
    host: 0.0.0.0
    port: <MESSAGE_PORT>
    role: normal
}

sslProfile {
    name: skupper-amqps
    certFile: /etc/qpid-dispatch-certs/skupper-amqps/tls.crt
    privateKeyFile: /etc/qpid-dispatch-certs/skupper-amqps/tls.key
    caCertFile: /etc/qpid-dispatch-certs/skupper-amqps/ca.crt
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
    name: skupper-internal
    certFile: /etc/qpid-dispatch-certs/skupper-internal/tls.crt
    privateKeyFile: /etc/qpid-dispatch-certs/skupper-internal/tls.key
    caCertFile: /etc/qpid-dispatch-certs/skupper-internal/ca.crt
}

listener {
    role: inter-router
    host: 0.0.0.0
    port: <INTERIOR_PORT>
    #sslProfile: skupper-internal
    #saslMechanisms: EXTERNAL
    #authenticatePeer: true
    saslMechanisms: ANONYMOUS
    authenticatePeer: no
}

listener {
    role: edge
    host: 0.0.0.0
    port: <EDGE_PORT>
    #sslProfile: skupper-internal
    #saslMechanisms: EXTERNAL
    #authenticatePeer: true
    saslMechanisms: ANONYMOUS
    authenticatePeer: no
}

address {
    prefix: mc
    distribution: multicast
}

`
