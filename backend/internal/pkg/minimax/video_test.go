package minimax

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVideoClientCreateAndQuery(t *testing.T) {
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
			if body.Model != "MiniMax-H3" || body.Content[0].Text != "A sunrise" || body.Ratio != "16:9" {
				t.Errorf("unexpected create body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"task_id":"12345"}`))
		case "/v2/query/video_generation/12345":
			if r.Method != http.MethodGet {
				t.Errorf("unexpected query method: %s", r.Method)
			}
			_, _ = w.Write([]byte(`{"task":{"id":"12345","model":"MiniMax-H3","status":"succeeded","content":{"url":"https://example.com/video.mp4"},"duration":5}}`))
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
		Model: "MiniMax-H3", Resolution: "768P", Duration: 5, Ratio: "16:9",
		Content: []VideoContent{{Type: "text", Text: "A sunrise"}},
	})
	if err != nil || created.TaskID != "12345" {
		t.Fatalf("create: result=%+v err=%v", created, err)
	}
	queried, err := client.Query(context.Background(), "test-key", created.TaskID)
	if err != nil || queried.Task.Status != "succeeded" || queried.Task.Content.URL == "" {
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
		t.Fatal("H3 Max is outside the H3 adapter scope")
	}
}
