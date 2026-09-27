package service

import (
	"net/url"
	"strings"
)

var seedreamModelIDs = []string{
	"doubao-seedream-5-0-pro-260628",
	"doubao-seedream-5-0-flash-260915",
	"doubao-seedream-5-0-260128",
	"doubao-seedream-4-5-251128",
	"doubao-seedream-4-0-250828",
}

func SeedreamModelIDs() []string {
	return append([]string(nil), seedreamModelIDs...)
}

func IsVolcengineArkAccount(account *Account) bool {
	if account == nil || !account.IsOpenAIApiKey() {
		return false
	}
	parsed, err := url.Parse(strings.TrimSpace(account.GetOpenAIBaseURL()))
	return err == nil && strings.EqualFold(parsed.Hostname(), "ark.cn-beijing.volces.com")
}
