# Design QA — 品牌标题与侧栏锁定组合

## Evidence

- Source visual truth:
  - `C:/Users/哈哈/AppData/Local/Temp/codex-clipboard-b58e7ab8-0fef-4b9f-afb8-05ec56cea983.png`（首页 Hero，510 × 565 px）
  - `C:/Users/哈哈/AppData/Local/Temp/codex-clipboard-a1103abb-7cd2-49ab-b182-953ce71716c9.png`（侧栏品牌区，249 × 93 px）
- Implementation:
  - `http://127.0.0.1:4173/`，Codex IAB 同轮内联截图（默认桌面视口与 510 × 565 CSS 响应式视口）
  - `http://127.0.0.1:4173/sidebar-brand-preview.html`，Codex IAB 同轮内联截图（256 × 96 CSS 聚焦预览；使用与组件相同的尺寸、间距和 Logo 处理）
- State: 浅色、首页未登录状态；侧栏为展开状态，默认品牌名与 `vedge` 版本徽标。
- Density normalization: 源图与浏览器截图均按 CSS 像素进行视觉比较；IAB 输出画布密度差异不作为问题。

## Full-view comparison

- 首页保留原有蓝白色调、大幅 star-X 标识、按钮与内容节奏，只替换品牌标题层级。
- 新标题由大幅 `star-X` 与下方 `API 算力中转站` 共同组成完整品牌名，不再重复显示旧的“你的私人 AI API 网关”。
- 桌面布局继续保持左侧品牌信息、右侧 API 示例面板；移动布局自动居中。

## Focused-region comparison

- 首页标题区：字体粗细、蓝色强调、标题与说明间距均与原设计体系一致，文案已更新。
- 侧栏品牌区：源图中的单行长标题会挤压 Logo；新布局将文字拆为 `star-X` 与 `API算力中转站` 两层，并将 Logo 可视尺寸提高约三分之一。
- Logo 使用原始高分辨率透明资产，以 `object-fit: contain` 配合受控缩放展示，没有生成或替换品牌资产。

## Required fidelity surfaces

- Fonts and typography: 沿用现有 Inter、苹方、微软雅黑回退体系；主副标题字重和行高形成清晰层级，无截断。
- Spacing and layout rhythm: 桌面 Hero 节奏保持不变；侧栏 Logo、两行文字和版本徽标在 64 px 高度内完整容纳。
- Colors and visual tokens: 沿用现有蓝色强调、灰色辅助文字和深色模式颜色。
- Image quality and asset fidelity: 沿用 `/starx-logo-transparent.png` 原始 1254 × 1254 RGBA 品牌资产；避免 `object-cover` 缩小裁切导致的模糊观感。
- Copy and content: 首页旧标题已移除，品牌语义统一为“star-X API算力中转站”。

## Comparison history

1. Initial finding — P2: 510 px 响应式视口中，新的弹性标题保持左对齐，与其余居中 Hero 内容不一致。
2. Fix: 在 900 px 以下为 `.hero-product-title` 增加居中对齐。
3. Post-fix evidence: 第二次同尺寸 IAB 对比中，Logo、标题、说明、按钮和能力列表均在同一中轴线上。

## Interaction and console checks

- Tested the Hero API tab interaction: switching to `Responses` updates the response preview to `Responses message routed.`。
- Console only reports the expected local public-settings request failure because the standalone Vite preview has no backend proxy target; the page and primary interaction render normally。

## Findings

- No remaining P0/P1/P2 findings.
- P3: 实际后台侧栏需要登录态才能在本地完整捕获；本次使用同尺寸、同资产、同 CSS 数值的聚焦预览核对品牌组合。

## Implementation checklist

- [x] 首页品牌标题统一
- [x] 移动端标题居中
- [x] 侧栏双行品牌文字
- [x] 侧栏 Logo 清晰度与可视尺寸优化
- [x] 主交互与控制台检查

final result: passed
