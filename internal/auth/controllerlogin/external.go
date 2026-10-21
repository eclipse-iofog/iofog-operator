package controllerlogin

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	iofogclient "github.com/eclipse-iofog/iofog-go-sdk/v3/pkg/client"
)

type oidcConfiguration struct {
	TokenEndpoint string `json:"token_endpoint"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
}

// ExternalClientCredentialsLogin discovers the token endpoint from issuerUrl OIDC metadata
// and obtains an access token via OAuth2 client_credentials grant.
// issuerCa is a Base64 encoded PEM CA bundle. Empty uses the system trust store.
// allowHTTP permits an http issuer and token endpoint when auth.insecureAllowHttp is set.
func ExternalClientCredentialsLogin(clt *iofogclient.Client, issuerURL, clientID, clientSecret, issuerCa string, allowHTTP bool) error {
	if err := validateIssuerScheme(issuerURL, allowHTTP); err != nil {
		return err
	}
	tlsConfig, err := issuerTLSConfig(issuerCa)
	if err != nil {
		return err
	}

	tokenEndpoint, err := discoverTokenEndpoint(issuerURL, allowHTTP, tlsConfig)
	if err != nil {
		return err
	}

	body := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
	}

	req, err := http.NewRequest(http.MethodPost, tokenEndpoint, strings.NewReader(body.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := httpClient(tlsConfig).Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusBadRequest {
		return fmt.Errorf("invalid client credentials")
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	var response tokenResponse
	if err := json.Unmarshal(resBody, &response); err != nil {
		return fmt.Errorf("parse token response: %w", err)
	}
	if response.AccessToken == "" {
		return fmt.Errorf("token response missing access token")
	}

	clt.SetAccessToken(response.AccessToken)
	return nil
}

func discoverTokenEndpoint(issuerURL string, allowHTTP bool, tlsConfig *tls.Config) (string, error) {
	discoveryURL, err := openIDConfigurationURL(issuerURL)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodGet, discoveryURL, nil)
	if err != nil {
		return "", err
	}

	res, err := httpClient(tlsConfig).Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OIDC discovery unexpected status code: %d", res.StatusCode)
	}

	var config oidcConfiguration
	if err := json.Unmarshal(resBody, &config); err != nil {
		return "", fmt.Errorf("parse OIDC discovery document: %w", err)
	}
	if config.TokenEndpoint == "" {
		return "", fmt.Errorf("OIDC discovery document missing token_endpoint")
	}
	if err := validateTokenEndpoint(issuerURL, config.TokenEndpoint, allowHTTP); err != nil {
		return "", err
	}
	return config.TokenEndpoint, nil
}

func validateIssuerScheme(issuerURL string, allowHTTP bool) error {
	parsed, err := url.Parse(strings.TrimSpace(issuerURL))
	if err != nil {
		return fmt.Errorf("parse issuer URL: %w", err)
	}
	return validateEndpointScheme(parsed.Scheme, allowHTTP, "issuer URL")
}

func validateTokenEndpoint(issuerURL, tokenEndpoint string, allowHTTP bool) error {
	issuer, err := url.Parse(strings.TrimSpace(issuerURL))
	if err != nil {
		return fmt.Errorf("parse issuer URL: %w", err)
	}
	if issuer.Hostname() == "" {
		return fmt.Errorf("issuer URL must include a host")
	}
	token, err := url.Parse(strings.TrimSpace(tokenEndpoint))
	if err != nil {
		return fmt.Errorf("parse token endpoint: %w", err)
	}
	if token.Hostname() == "" {
		return fmt.Errorf("token endpoint must include a host")
	}
	if !strings.EqualFold(issuer.Hostname(), token.Hostname()) || endpointPort(issuer) != endpointPort(token) {
		return fmt.Errorf("token endpoint host %s does not match issuer host %s", token.Host, issuer.Host)
	}
	return validateEndpointScheme(token.Scheme, allowHTTP, "token endpoint")
}

func validateEndpointScheme(scheme string, allowHTTP bool, what string) error {
	switch strings.ToLower(scheme) {
	case "https":
		return nil
	case "http":
		if !allowHTTP {
			return fmt.Errorf("%s uses http but auth.insecureAllowHttp is not set", what)
		}
		return nil
	default:
		return fmt.Errorf("%s scheme %q is not allowed", what, scheme)
	}
}

func endpointPort(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		return "443"
	case "http":
		return "80"
	default:
		return ""
	}
}

func openIDConfigurationURL(issuerURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(issuerURL))
	if err != nil {
		return "", fmt.Errorf("parse issuer URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("issuer URL must include scheme and host")
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/") + "/.well-known/openid-configuration"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}
