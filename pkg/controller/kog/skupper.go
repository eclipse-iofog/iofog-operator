package kog

const routerConfig = `
router {
    mode: interior
    id: Router.A
}

listener {
    host: localhost
    port: 5672
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
    port: 5671
    role: normal
    #sslProfile: skupper-amqps
    #saslMechanisms: EXTERNAL
    #authenticatePeer: true
    saslMechanisms: ANONYMOUS
    authenticatePeer: no
}

#{{- if eq .Console "internal"}}
#listener {
#    host: 0.0.0.0
#    port: 8080
#    role: normal
#    http: true
#    authenticatePeer: true
#}
#{{- else if eq .Console "unsecured"}}
#listener {
#    host: 0.0.0.0
#    port: 8080
#    role: normal
#    http: true
#}
#{{- end }}

listener {
    host: 0.0.0.0
    port: 9090
    role: normal
    http: true
    httpRootDir: disabled
    websockets: false
    healthz: true
    metrics: true
}

#{{- if eq .Mode "interior" }}
sslProfile {
    name: skupper-internal
    certFile: /etc/qpid-dispatch-certs/skupper-internal/tls.crt
    privateKeyFile: /etc/qpid-dispatch-certs/skupper-internal/tls.key
    caCertFile: /etc/qpid-dispatch-certs/skupper-internal/ca.crt
}

listener {
    role: inter-router
    host: 0.0.0.0
    port: 55671
    #sslProfile: skupper-internal
    #saslMechanisms: EXTERNAL
    #authenticatePeer: true
    saslMechanisms: ANONYMOUS
    authenticatePeer: no
}

listener {
    role: edge
    host: 0.0.0.0
    port: 45671
    #sslProfile: skupper-internal
    #saslMechanisms: EXTERNAL
    #authenticatePeer: true
    saslMechanisms: ANONYMOUS
    authenticatePeer: no
}
#{{- end}}

address {
    prefix: mc
    distribution: multicast
}

## Connectors: ##
`
