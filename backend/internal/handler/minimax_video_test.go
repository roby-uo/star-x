//go:build unit

package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/minimax"
	"github.com/stretchr/testify/require"
)

func TestMiniMaxPublicVideoContentBillingBoundary(t *testing.T) {
	text := minimax.VideoContent{Type: "text", Text: "sunrise"}
	first := minimax.VideoContent{Type: "image_url", Role: "first_frame", ImageURL: &minimax.MediaURL{URL: "https://example.com/first.png"}}
	last := minimax.VideoContent{Type: "image_url", Role: "last_frame", ImageURL: &minimax.MediaURL{URL: "https://example.com/last.png"}}
	reference := minimax.VideoContent{Type: "video_url", Role: "reference_video", VideoURL: &minimax.MediaURL{URL: "https://example.com/ref.mp4"}}
	require.True(t, miniMaxPublicContentAllowed([]minimax.VideoContent{text, first, last}))
	require.False(t, miniMaxPublicContentAllowed([]minimax.VideoContent{text, first, first}))
	require.False(t, miniMaxPublicContentAllowed([]minimax.VideoContent{text, reference}))
}
