// Автор: Kuruma
package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"holo-site-backend/internal/config"
)

type SupabaseRESTClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type SupabaseRequestError struct {
	Status  int
	Code    string
	Message string
	Details string
	Hint    string
}

type supabaseErrorPayload struct {
	Code             string `json:"code"`
	Message          string `json:"message"`
	Details          any    `json:"details"`
	Hint             any    `json:"hint"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func NewSupabaseRESTClient(cfg config.SupabaseConfig) (*SupabaseRESTClient, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.URL), "/")
	apiKey := strings.TrimSpace(cfg.ServerKey())
	if baseURL == "" || apiKey == "" {
		return nil, fmt.Errorf("supabase rest configuration is incomplete")
	}

	return &SupabaseRESTClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}, nil
}

func (c *SupabaseRESTClient) Probe(ctx context.Context) error {
	values := url.Values{}
	values.Set("select", "id")
	values.Set("limit", "1")

	var rows []map[string]any
	if err := c.Get(ctx, "store_catalog_items", values, &rows); err != nil {
		if apiErr, ok := err.(*SupabaseRequestError); ok && apiErr.Code == "PGRST205" {
			return fmt.Errorf("supabase schema is not applied yet: %w", err)
		}
		return err
	}

	return nil
}

func (c *SupabaseRESTClient) Get(ctx context.Context, resource string, query url.Values, dest any) error {
	return c.doJSON(ctx, http.MethodGet, resource, query, nil, "", dest)
}

func (c *SupabaseRESTClient) Post(ctx context.Context, resource string, query url.Values, payload any, dest any) error {
	return c.doJSON(ctx, http.MethodPost, resource, query, payload, "return=representation", dest)
}

func (c *SupabaseRESTClient) Patch(ctx context.Context, resource string, query url.Values, payload any, dest any) error {
	return c.doJSON(ctx, http.MethodPatch, resource, query, payload, "return=representation", dest)
}

func (c *SupabaseRESTClient) Delete(ctx context.Context, resource string, query url.Values) error {
	return c.doJSON(ctx, http.MethodDelete, resource, query, nil, "return=minimal", nil)
}

func (c *SupabaseRESTClient) doJSON(ctx context.Context, method, resource string, query url.Values, payload any, prefer string, dest any) error {
	endpoint := c.baseURL + "/rest/v1/" + strings.TrimLeft(resource, "/")
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}

	request.Header.Set("apikey", c.apiKey)
	request.Header.Set("Authorization", "Bearer "+c.apiKey)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Profile", "public")
	request.Header.Set("Content-Profile", "public")
	request.Header.Set("User-Agent", "estelar-backend/1.0")
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if prefer != "" {
		request.Header.Set("Prefer", prefer)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusBadRequest {
		return decodeSupabaseError(response)
	}

	if dest == nil || response.StatusCode == http.StatusNoContent {
		_, _ = io.Copy(io.Discard, response.Body)
		return nil
	}

	if err := json.NewDecoder(response.Body).Decode(dest); err != nil {
		return err
	}

	return nil
}

func decodeSupabaseError(response *http.Response) error {
	body, _ := io.ReadAll(response.Body)
	payload := supabaseErrorPayload{}
	_ = json.Unmarshal(body, &payload)

	message := strings.TrimSpace(payload.Message)
	if message == "" {
		message = strings.TrimSpace(payload.ErrorDescription)
	}
	if message == "" {
		message = strings.TrimSpace(payload.Error)
	}
	if message == "" {
		message = http.StatusText(response.StatusCode)
	}

	return &SupabaseRequestError{
		Status:  response.StatusCode,
		Code:    strings.TrimSpace(payload.Code),
		Message: message,
		Details: stringifySupabaseValue(payload.Details),
		Hint:    stringifySupabaseValue(payload.Hint),
	}
}

func (e *SupabaseRequestError) Error() string {
	parts := []string{fmt.Sprintf("supabase request failed (%d)", e.Status)}
	if e.Code != "" {
		parts = append(parts, e.Code)
	}
	if e.Message != "" {
		parts = append(parts, e.Message)
	}
	if e.Hint != "" {
		parts = append(parts, "hint: "+e.Hint)
	}
	return strings.Join(parts, ": ")
}

func stringifySupabaseValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}
