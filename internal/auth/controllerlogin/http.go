package controllerlogin

import (
	"crypto/tls"
	"net/http"
)

func insecureHTTPTransport() *http.Transport {
	return &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // IdP/Controller commonly use self-signed TLS
	}
}
