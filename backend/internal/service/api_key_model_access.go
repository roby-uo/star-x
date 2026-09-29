package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

// A key may only narrow group access. Nil model whitelist means unrestricted;
// an enabled empty whitelist deliberately denies all models.
type APIKeyModelAccess struct {
	RestrictModels   bool     `json:"restrict_models"`
	Models           []string `json:"models"`
	VideoResolutions []string `json:"video_resolutions"`
	VideoMaxDuration int      `json:"video_max_duration"`
}

func (s *MediaTaskService) KeyAccess(ctx context.Context, keyID int64) (*APIKeyModelAccess, error) {
	var raw []byte
	err := s.db.QueryRowContext(ctx, `SELECT rules FROM api_key_model_access WHERE api_key_id=$1`, keyID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return &APIKeyModelAccess{}, nil
	}
	if err != nil {
		return nil, err
	}
	var rules APIKeyModelAccess
	err = json.Unmarshal(raw, &rules)
	return &rules, err
}
func (s *MediaTaskService) SaveKeyAccess(ctx context.Context, keyID int64, rules APIKeyModelAccess) error {
	if len(rules.Models) > 200 || rules.VideoMaxDuration < 0 || rules.VideoMaxDuration > 15 {
		return fmt.Errorf("密钥权限配置无效")
	}
	for _, model := range rules.Models {
		if len(model) == 0 || len(model) > 200 {
			return fmt.Errorf("模型名称无效")
		}
	}
	for _, res := range rules.VideoResolutions {
		if !slices.Contains([]string{"480P", "720P", "768P", "1080P", "2K"}, res) {
			return fmt.Errorf("视频分辨率无效")
		}
	}
	raw, err := json.Marshal(rules)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO api_key_model_access(api_key_id,rules) VALUES($1,$2) ON CONFLICT(api_key_id) DO UPDATE SET rules=$2,updated_at=NOW()`, keyID, raw)
	return err
}
func (a *APIKeyModelAccess) Validate(model, resolution string, duration int) error {
	if a == nil {
		return nil
	}
	if a.RestrictModels && !slices.Contains(a.Models, model) {
		return fmt.Errorf("此 API 密钥未授权调用该模型")
	}
	if duration > 0 && a.VideoMaxDuration > 0 && duration > a.VideoMaxDuration {
		return fmt.Errorf("视频时长超过此密钥限制")
	}
	if duration > 0 && len(a.VideoResolutions) > 0 && !slices.Contains(a.VideoResolutions, resolution) {
		return fmt.Errorf("视频分辨率超出此密钥权限")
	}
	return nil
}
