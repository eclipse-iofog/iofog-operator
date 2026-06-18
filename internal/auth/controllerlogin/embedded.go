package controllerlogin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	iofogclient "github.com/eclipse-iofog/iofog-go-sdk/v3/pkg/client"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

// EmbeddedBootstrapLogin authenticates against Controller embedded auth via POST /user/login.
func EmbeddedBootstrapLogin(clt *iofogclient.Client, username, password string) error {
	loginURL, err := userLoginURL(clt.GetBaseURL())
	if err != nil {
		return err
	}

	body, err := json.Marshal(loginRequest{Email: username, Password: password})
	if err != nil {
		return fmt.Errorf("marshal login request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, loginURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

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
		return fmt.Errorf("invalid credentials")
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	var response loginResponse
	if err := json.Unmarshal(resBody, &response); err != nil {
		return fmt.Errorf("parse login response: %w", err)
	}
	if response.AccessToken == "" {
		return fmt.Errorf("login response missing access token")
	}

	clt.SetAccessToken(response.AccessToken)
	if response.RefreshToken != "" {
		clt.SetRefreshToken(response.RefreshToken)
	}
	return nil
}

func userLoginURL(baseURL string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse controller base URL: %w", err)
	}
	parsed.Path = path.Join(parsed.Path, "user", "login")
	return strings.TrimSuffix(parsed.String(), "/"), nil
}
