package handler

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/minimax"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// The opaque quote version still binds generation to a reviewed specification;
// monetary snapshots stay on the server for settlement and administrator review.
type userVideoQuote struct {
	Version string `json:"version"`
}

func userQuoteFromVideoQuote(q *service.VideoQuote) *userVideoQuote {
	if q == nil {
		return nil
	}
	return &userVideoQuote{Version: q.Version}
}

type userMediaTask struct {
	ID             string                      `json:"id"`
	UserID         int64                       `json:"user_id"`
	APIKeyID       int64                       `json:"api_key_id"`
	GroupID        int64                       `json:"group_id"`
	Model          string                      `json:"model"`
	UpstreamTaskID string                      `json:"task_id,omitempty"`
	State          string                      `json:"state"`
	Specification  minimax.CreateVideoRequest  `json:"specification"`
	Result         *minimax.QueryVideoResponse `json:"result,omitempty"`
	Error          string                      `json:"error,omitempty"`
	CreatedAt      time.Time                   `json:"created_at"`
}

func userMediaTaskFromService(task *service.MediaTask) *userMediaTask {
	if task == nil {
		return nil
	}
	return &userMediaTask{task.ID, task.UserID, task.APIKeyID, task.GroupID, task.Model, task.UpstreamTaskID, task.State, task.Specification, task.Result, task.Error, task.CreatedAt}
}
