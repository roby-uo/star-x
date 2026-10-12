package service

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/minimax"
)

// China API list prices, maintained with the same dated supplier template used
// by administrators. These are CNY prices, never the global USD H3 price table.
func miniMaxChinaSimplePrices(model, modality, resolution, tier string) []CostPriceRow {
	name := strings.ToLower(strings.TrimSpace(model))
	number := func(value float64) *float64 { return &value }
	row := func(input, output, read, write float64) CostPriceRow {
		return CostPriceRow{ID: "minimax-cn-20261012", Models: []string{model}, Modality: modality, BillingUnit: "per_million_tokens", InputPrice: number(input), OutputPrice: number(output), CacheReadPrice: number(read), CacheWritePrice: number(write), ServiceTier: tier}
	}
	if modality == "video" && tier == "" {
		price := -1.0
		switch name {
		case "minimax-h3":
			switch strings.ToUpper(resolution) {
			case "768P":
				price = .5
			case "2K":
				price = .8
			}
		case "minimax-h3-max":
			switch strings.ToUpper(resolution) {
			case "480P":
				price = .33
			case "768P":
				price = .5
			}
		}
		if price >= 0 {
			return []CostPriceRow{{ID: "minimax-cn-20261012-video", Models: []string{model}, Modality: modality, BillingUnit: "per_second", Resolution: resolution, UnitPrice: number(price)}}
		}
		return nil
	}
	if modality != "chat" {
		return nil
	}
	if name == "minimax-m3" && (tier == "" || tier == "priority") {
		factor := 1.0
		if tier == "priority" {
			factor = 1.5
		}
		low, high := row(2.1*factor, 8.4*factor, .42*factor, 0), row(4.2*factor, 16.8*factor, .84*factor, 0)
		upper := 512000
		low.MaxTokens = &upper
		high.MinTokens = upper
		return []CostPriceRow{low, high}
	}
	if tier != "" {
		return nil
	}
	switch name {
	case "minimax-m2.7":
		return []CostPriceRow{row(2.1, 8.4, .42, 2.625)}
	case "minimax-m2.7-highspeed":
		return []CostPriceRow{row(4.2, 8.4, .42, 2.625)}
	case "minimax-m2.5", "minimax-m2.1", "minimax-m2":
		return []CostPriceRow{row(2.1, 8.4, .21, 2.625)}
	case "minimax-m2.5-highspeed", "minimax-m2.1-highspeed":
		return []CostPriceRow{row(4.2, 8.4, .21, 2.625)}
	}
	return nil
}

func miniMaxSimpleGlobalVideoPrice(model, resolution string) (float64, bool) {
	spec, ok := minimax.VideoModelSpecFor(model)
	if !ok {
		return 0, false
	}
	price, ok := spec.PricesUSD[strings.ToUpper(strings.TrimSpace(resolution))]
	return price, ok
}
