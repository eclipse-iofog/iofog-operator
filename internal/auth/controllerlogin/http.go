package controllerlogin

import (
	"crypto/tls"
	"net/http"
)

func insecureHTTPTransport() *http.Transport {
	return &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // #nosec G402 -- Controller/IdP commonly use self-signed TLS
	}
}
