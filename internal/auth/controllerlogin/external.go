package controllerlogin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

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
func ExternalClientCredentialsLogin(clt *iofogclient.Client, issuerURL, clientID, clientSecret string) error {
	tokenEndpoint, err := discoverTokenEndpoint(issuerURL)
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

	res, err := httpClient().Do(req)
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

func discoverTokenEndpoint(issuerURL string) (string, error) {
	discoveryURL, err := openIDConfigurationURL(issuerURL)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodGet, discoveryURL, nil)
	if err != nil {
		return "", err
	}

	res, err := httpClient().Do(req)
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
	return config.TokenEndpoint, nil
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

func httpClient() *http.Client {
	return &http.Client{
		Transport: insecureHTTPTransport(),
		Timeout:   10 * time.Second,
	}
}
