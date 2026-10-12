package service

import "strings"

// Maintained CNY reference tables. A fixed published peak reference for DeepSeek
// avoids silently varying a customer's rate by upstream time-of-day discounts.
// Source: https://api-docs.deepseek.com/zh-cn/quick_start/pricing/ (2026-10-12)
// Source: https://docs.volcengine.com/docs/ark/model-pricing?lang=zh (2026-10-12)
// Source: https://docs.bigmodel.cn/cn/guide/start/pricing.md (2026-10-12)
// Source: https://platform.kimi.com/docs/pricing/chat.md (2026-10-12)
// Source: https://platform.kimi.com/docs/guide/use-kimi-vision-model.md (2026-10-12)
func chinaSimplePrices(provider, model, modality, resolution, tier, scene string) []CostPriceRow {
	if tier != "" {
		return nil
	}
	number := func(v float64) *float64 { return &v }
	name := strings.ToLower(strings.TrimSpace(model))
	token := func(input, output, read float64) []CostPriceRow {
		return []CostPriceRow{{ID: provider + "-cn-reference-20261012", Models: []string{model}, Modality: modality, BillingUnit: "per_million_tokens", InputPrice: number(input), OutputPrice: number(output), CacheReadPrice: number(read), CacheWritePrice: number(0)}}
	}
	if provider == "deepseek" && modality == "chat" {
		switch name {
		case "deepseek-flash", "deepseek-v4-flash", "deepseek-v4-flash-vision-exp":
			return token(2, 8, .04)
		case "deepseek-v4-pro":
			return token(9, 27, .3)
		}
	}
	if provider == "zhipu" && modality == "chat" {
		// Official CNY standard references read from the public pricing page on
		// 2026-10-12. Flash promotion and cache storage holidays are not promised
		// as a permanent contract price; administrators can bind an override.
		// https://open.bigmodel.cn/pricing
		switch name {
		case "glm-5.2", "glm-5.3":
			return token(8, 28, 2)
		case "glm-5.3-flash":
			rows := token(.8, 2.8, .23)
			// The official table lists one input price for text, image, video
			// and file inputs. No separate premium is inferred for vision.
			rows[0].ImageInputPrice = number(.8)
			return rows
		case "glm-5.3-flashx":
			rows := token(2, 7, .57)
			rows[0].ImageInputPrice = number(2)
			return rows
		}
	}
	if provider == "moonshot" && modality == "chat" {
		var rows []CostPriceRow
		switch name {
		case "kimi-k2.6":
			rows = token(6.5, 27, 1.1)
		case "kimi-k2.7-code":
			rows = token(6.5, 27, 1.3)
		case "kimi-k2.7-code-highspeed":
			rows = token(13, 54, 2.6)
		default:
			return nil
		}
		// K2 vision is charged by total inference tokens under the same table.
		rows[0].ImageInputPrice = number(*rows[0].InputPrice)
		return rows
	}
	if provider == "volcengine" && modality == "image" {
		unit := -1.0
		switch name {
		case "doubao-seedream-4-0-250828":
			unit = .20
		case "doubao-seedream-4-5-251128":
			unit = .25
		case "doubao-seedream-5-0-260128", "doubao-seedream-5-0-lite-260128":
			unit = .22
		case "doubao-seedream-5-0-flash-260915":
			unit = .12
		case "doubao-seedream-5-0-pro-260628":
			// The Pro price is pixel-based, with a different layer-splitting
			// scene price. Adapters must explicitly identify the scene.
			if scene != "single_image" && scene != "layered" {
				return nil
			}
			low := false
			switch strings.ToUpper(strings.TrimSpace(resolution)) {
			case "1K", "1.5K":
				low = true
			case "2K":
				low = false
			default:
				width, height, ok := parseImageBillingDimensions(resolution)
				if !ok || width <= 0 || height <= 0 || width > 4624220 || height > 4624220 {
					return nil
				}
				pixels := int64(width) * int64(height)
				if pixels < 921600 || pixels > 4624220 {
					return nil
				}
				low = int64(width)*int64(height) <= 2610000
			}
			if low {
				unit = .30
			} else {
				unit = .60
			}
			if scene == "layered" {
				unit /= 2
			}
		}
		if unit >= 0 {
			row := CostPriceRow{ID: "seedream-cn-reference-20261012", Models: []string{model}, Modality: modality, BillingUnit: "per_image", UnitPrice: number(unit)}
			if name == "doubao-seedream-5-0-pro-260628" {
				row.InputImageUnitPrice = number(.02)
			}
			return []CostPriceRow{row}
		}
	}
	if provider == "volcengine" && modality == "chat" && name == "doubao-embedding-vision" {
		rows := token(.7, 0, 0)
		rows[0].ImageInputPrice = number(1.8)
		return rows
	}
	return nil
}
