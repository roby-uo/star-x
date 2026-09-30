package minimax

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVideoClientCreateAndQuery(t *testing.T) {
	for _, model := range []string{"MiniMax-H3", "MiniMax-H3-Max"} {
		t.Run(model, func(t *testing.T) { testVideoClientCreateAndQuery(t, model) })
	}
}

func testVideoClientCreateAndQuery(t *testing.T, model string) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing API key header")
		}
		switch r.URL.Path {
		case "/v2/video_generation":
			if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("unexpected create request: %s %s", r.Method, r.Header.Get("Content-Type"))
			}
			var body CreateVideoRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Model != model || body.Content[0].Text != "A sunrise" || body.Ratio != "16:9" {
				t.Errorf("unexpected create body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"task_id":"12345"}`))
		case "/v2/query/video_generation/12345":
			if r.Method != http.MethodGet {
				t.Errorf("unexpected query method: %s", r.Method)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"task": map[string]any{"id": "12345", "model": model, "status": "succeeded", "content": map[string]any{"url": "https://example.com/video.mp4"}, "duration": 5}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := NewVideoClient(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	created, err := client.Create(context.Background(), "test-key", CreateVideoRequest{
		Model: model, Resolution: "768P", Duration: 5, Ratio: "16:9",
		Content: []VideoContent{{Type: "text", Text: "A sunrise"}},
	})
	if err != nil || created.TaskID != "12345" {
		t.Fatalf("create: result=%+v err=%v", created, err)
	}
	queried, err := client.Query(context.Background(), "test-key", created.TaskID)
	if err != nil || queried.Task.Status != "succeeded" || queried.Task.Model != model || queried.Task.Content.URL == "" {
		t.Fatalf("query: result=%+v err=%v", queried, err)
	}
}

func TestCreateVideoRequestValidation(t *testing.T) {
	valid := CreateVideoRequest{
		Model: "MiniMax-H3", Resolution: "2K", Duration: 5, Ratio: "9:16",
		Content: []VideoContent{{Type: "text", Text: "A sunrise"}},
	}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	invalid := valid
	invalid.Duration = 16
	if err := invalid.Validate(); err == nil {
		t.Fatal("duration 16 should be rejected")
	}
	invalid = valid
	invalid.Ratio = "adaptive"
	if err := invalid.Validate(); err == nil {
		t.Fatal("text-to-video adaptive ratio should be rejected")
	}
	invalid = valid
	invalid.Model = "MiniMax-H3-Max"
	if err := invalid.Validate(); err == nil {
		t.Fatal("H3 Max must reject 2K")
	}
}

func TestVideoModelSpecificSpecifications(t *testing.T) {
	for _, tc := range []struct {
		model, resolution string
		duration          int
		valid             bool
	}{
		{"MiniMax-H3", "768P", 4, true},
		{"MiniMax-H3", "2K", 15, true},
		{"MiniMax-H3", "480P", 5, false},
		{"MiniMax-H3-Max", "480P", 5, true},
		{"MiniMax-H3-Max", "768P", 15, true},
		{"MiniMax-H3-Max", "480P", 4, false},
		{"MiniMax-H3-Max", "2K", 5, false},
		{"MiniMax-H3-Max", "768P", 16, false},
		{"MiniMax-H3-Context-IR", "768P", 5, false},
	} {
		input := CreateVideoRequest{Model: tc.model, Resolution: tc.resolution, Duration: tc.duration, Ratio: "16:9", Content: []VideoContent{{Type: "text", Text: "sunrise"}}}
		if err := input.Validate(); (err == nil) != tc.valid {
			t.Errorf("%s %s %ds: expected valid=%v, error=%v", tc.model, tc.resolution, tc.duration, tc.valid, err)
		}
	}
}
