package service

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// MiniMaxVideoBaseURL accepts only the official MiniMax video API hosts. A
// dedicated host check prevents a generic OpenAI account from receiving H3 jobs.
func MiniMaxVideoBaseURL(account *Account) (string, bool) {
	if account == nil || account.Platform != PlatformOpenAI || account.Type != AccountTypeAPIKey || strings.TrimSpace(account.GetCredential("api_key")) == "" {
		return "", false
	}
	raw := strings.TrimSpace(account.GetCredential("base_url"))
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", false
	}
	if u.Hostname() != "api.minimax.cn" && u.Hostname() != "api.minimax.io" {
		return "", false
	}
	if u.Port() != "" || (u.Path != "" && u.Path != "/" && u.Path != "/v1" && u.Path != "/v1/") {
		return "", false
	}
	return u.Scheme + "://" + u.Hostname(), true
}

func miniMaxVideoBindingKey(taskID string, userID, apiKeyID int64) string {
	if strings.TrimSpace(taskID) == "" || userID <= 0 || apiKeyID <= 0 {
		return ""
	}
	return "minimax-h3-video:" + DeriveSessionHashFromSeed(fmt.Sprintf("%d:%d:%s", userID, apiKeyID, taskID))
}

func (s *OpenAIGatewayService) BindMiniMaxVideoTaskAccount(ctx context.Context, groupID *int64, taskID string, userID, apiKeyID, accountID int64) error {
	if s == nil || s.cache == nil || accountID <= 0 {
		return fmt.Errorf("MiniMax video task binding unavailable")
	}
	key := s.openAISessionCacheKey(miniMaxVideoBindingKey(taskID, userID, apiKeyID))
	if key == "" {
		return fmt.Errorf("MiniMax video task binding invalid")
	}
	return s.cache.SetSessionAccountID(ctx, derefGroupID(groupID), key, accountID, 7*24*time.Hour)
}

func (s *OpenAIGatewayService) ResolveMiniMaxVideoTaskAccount(ctx context.Context, groupID *int64, taskID string, userID, apiKeyID int64) (*Account, error) {
	if s == nil || s.cache == nil || s.accountRepo == nil {
		return nil, fmt.Errorf("MiniMax video task lookup unavailable")
	}
	key := s.openAISessionCacheKey(miniMaxVideoBindingKey(taskID, userID, apiKeyID))
	if key == "" {
		return nil, fmt.Errorf("MiniMax video task lookup invalid")
	}
	accountID, err := s.cache.GetSessionAccountID(ctx, derefGroupID(groupID), key)
	if err != nil || accountID <= 0 {
		return nil, fmt.Errorf("MiniMax video task not found")
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil {
		return nil, fmt.Errorf("MiniMax video task account unavailable")
	}
	if _, ok := MiniMaxVideoBaseURL(account); !ok {
		return nil, fmt.Errorf("MiniMax video task account unavailable")
	}
	return account, nil
}
