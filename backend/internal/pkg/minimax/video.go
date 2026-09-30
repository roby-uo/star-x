package minimax

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

const DefaultVideoBaseURL = "https://api.minimax.cn"
const maxVideoResponseBytes = 2 << 20

// VideoModelSpec describes the generation models supported by this adapter.
// Prices are the global API's USD list prices per output second, not CN costs.
type VideoModelSpec struct {
	Resolutions []string
	MinDuration int
	MaxDuration int
	PricesUSD   map[string]float64
}

func VideoModelSpecFor(model string) (VideoModelSpec, bool) {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "minimax-h3":
		return VideoModelSpec{Resolutions: []string{"768P", "2K"}, MinDuration: 4, MaxDuration: 15, PricesUSD: map[string]float64{"768P": .08, "2K": .13}}, true
	case "minimax-h3-max":
		return VideoModelSpec{Resolutions: []string{"480P", "768P"}, MinDuration: 5, MaxDuration: 15, PricesUSD: map[string]float64{"480P": .05, "768P": .08}}, true
	default:
		return VideoModelSpec{}, false
	}
}

func IsVideoModel(model string) bool {
	_, ok := VideoModelSpecFor(model)
	return ok
}

type MediaURL struct {
	URL string `json:"url"`
}

type VideoContent struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	Role     string    `json:"role,omitempty"`
	ImageURL *MediaURL `json:"image_url,omitempty"`
	VideoURL *MediaURL `json:"video_url,omitempty"`
	AudioURL *MediaURL `json:"audio_url,omitempty"`
}

type CreateVideoRequest struct {
	Model         string         `json:"model"`
	Content       []VideoContent `json:"content"`
	Resolution    string         `json:"resolution"`
	Duration      int            `json:"duration"`
	Ratio         string         `json:"ratio,omitempty"`
	AIGCWatermark *bool          `json:"aigc_watermark,omitempty"`
}

type CreateVideoResponse struct {
	TaskID string `json:"task_id"`
}

type VideoTask struct {
	ID         string `json:"id"`
	Model      string `json:"model"`
	Status     string `json:"status"`
	Resolution string `json:"resolution"`
	Duration   int    `json:"duration"`
	Ratio      string `json:"ratio"`
	Content    struct {
		URL string `json:"url"`
	} `json:"content"`
	Usage struct {
		TotalSeconds  int `json:"total_seconds"`
		OutputSeconds int `json:"output_seconds"`
	} `json:"usage"`
}

type QueryVideoResponse struct {
	Task VideoTask `json:"task"`
}

type HTTPError struct{ Status int }

func (e *HTTPError) Error() string {
	return fmt.Sprintf("MiniMax video API returned HTTP %d", e.Status)
}

type VideoClient struct {
	baseURL string
	http    *http.Client
}

func NewVideoClient(baseURL string, httpClient *http.Client) (*VideoClient, error) {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultVideoBaseURL
	}
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" || parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("invalid MiniMax video base URL")
	}
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	client := *httpClient
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &VideoClient{baseURL: strings.TrimRight(parsed.String(), "/"), http: &client}, nil
}

func (r CreateVideoRequest) Validate() error {
	if r.Model != "MiniMax-H3" && r.Model != "MiniMax-H3-Max" {
		return errors.New("model must be MiniMax-H3 or MiniMax-H3-Max")
	}
	spec, _ := VideoModelSpecFor(r.Model)
	if !slices.Contains(spec.Resolutions, r.Resolution) {
		return fmt.Errorf("%s resolution must be %s", r.Model, strings.Join(spec.Resolutions, " or "))
	}
	if r.Duration < spec.MinDuration || r.Duration > spec.MaxDuration {
		return fmt.Errorf("%s duration must be between %d and %d seconds", r.Model, spec.MinDuration, spec.MaxDuration)
	}
	hasPrompt := false
	hasMedia := false
	for _, item := range r.Content {
		switch item.Type {
		case "text":
			hasPrompt = hasPrompt || strings.TrimSpace(item.Text) != ""
		case "image_url":
			hasMedia = true
			if item.ImageURL == nil || strings.TrimSpace(item.ImageURL.URL) == "" {
				return errors.New("image_url is required for image content")
			}
		case "video_url":
			hasMedia = true
			if item.VideoURL == nil || strings.TrimSpace(item.VideoURL.URL) == "" {
				return errors.New("video_url is required for video content")
			}
		case "audio_url":
			hasMedia = true
			if item.AudioURL == nil || strings.TrimSpace(item.AudioURL.URL) == "" {
				return errors.New("audio_url is required for audio content")
			}
		default:
			return fmt.Errorf("unsupported content type %q", item.Type)
		}
	}
	if !hasPrompt {
		return errors.New("content must include a non-empty text prompt")
	}
	if !hasMedia && (r.Ratio == "" || r.Ratio == "adaptive") {
		return errors.New("text-to-video requires a non-adaptive ratio")
	}
	if r.Ratio != "" && r.Ratio != "adaptive" && r.Ratio != "21:9" && r.Ratio != "16:9" && r.Ratio != "4:3" && r.Ratio != "1:1" && r.Ratio != "3:4" && r.Ratio != "9:16" {
		return errors.New("unsupported video ratio")
	}
	return nil
}

func (c *VideoClient) Create(ctx context.Context, apiKey string, input CreateVideoRequest) (*CreateVideoResponse, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	data, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var result CreateVideoResponse
	if err := c.do(ctx, http.MethodPost, "/v2/video_generation", apiKey, data, &result); err != nil {
		return nil, err
	}
	if strings.TrimSpace(result.TaskID) == "" {
		return nil, errors.New("MiniMax video creation returned no task_id")
	}
	return &result, nil
}

func (c *VideoClient) Query(ctx context.Context, apiKey, taskID string) (*QueryVideoResponse, error) {
	if taskID == "" || strings.ContainsAny(taskID, "/?\\#") {
		return nil, errors.New("invalid MiniMax video task_id")
	}
	var result QueryVideoResponse
	if err := c.do(ctx, http.MethodGet, "/v2/query/video_generation/"+url.PathEscape(taskID), apiKey, nil, &result); err != nil {
		return nil, err
	}
	if result.Task.ID == "" {
		return nil, errors.New("MiniMax video query returned no task")
	}
	return &result, nil
}

func (c *VideoClient) do(ctx context.Context, method, path, apiKey string, body []byte, out any) error {
	if strings.TrimSpace(apiKey) == "" {
		return errors.New("MiniMax API key is required")
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("MiniMax video request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxVideoResponseBytes+1))
	if err != nil {
		return err
	}
	if len(responseBody) > maxVideoResponseBytes {
		return errors.New("MiniMax video response too large")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &HTTPError{Status: resp.StatusCode}
	}
	if err := json.Unmarshal(responseBody, out); err != nil {
		return fmt.Errorf("decode MiniMax video response: %w", err)
	}
	return nil
}
