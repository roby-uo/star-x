# MiniMax 语言模型与 H3 系列视频接入

star-X 通过 OpenAI 兼容账号管理 MiniMax API Key、模型与分组。语言模型使用聊天接口，H3 系列视频使用 MiniMax 原生异步任务接口；视频任务创建后，通过任务编号查询状态和结果。

## 当前视频能力

| 调用模型名 | 分辨率 | 时长 | 页面支持的生成方式 |
| --- | --- | --- | --- |
| `MiniMax-H3` | `768P`、`2K` | 4–15 秒 | 文生视频、首帧图生视频、首尾帧视频 |
| `MiniMax-H3-Max` | `480P`、`768P` | 5–15 秒 | 文生视频、首帧图生视频、首尾帧视频 |

使用表中的完整模型名称。管理员可以进一步收窄分辨率、时长和生成方式；用户密钥的权限也只能收窄所属分组的范围。H3-Max 不支持 `2K` 或 4 秒。

当前中转适配不开放参考视频、参考音频、参考图片、H3-Context-IR 或视频再生成。MiniMax 官方提供的能力可能比此处已接入的范围更多，不能仅凭官方模型清单推断中转站已经支持全部参数。

## 配置上游账号

1. 在管理端「上游管理」创建或编辑账号，平台选择 OpenAI，账号类型选择 API Key。
2. Base URL（上游服务地址）填写 `https://api.minimax.cn/v1`（中国站）或 `https://api.minimax.io/v1`（国际站），填入对应站点的 MiniMax API Key。视频转发会使用同一主机的 `/v2` 路径。
3. 语言模型可用「同步上游支持的模型」获取账号返回的列表，或选择已确认获授权的模型。当前预设包括 `MiniMax-M3`、`MiniMax-M2.7`、`MiniMax-M2.7-highspeed`、M2.5、M2.1 与 M2 系列；预设列表不代表账号全部可用。
4. 点击「添加 H3 / H3-Max 视频模型」追加两个视频模型，已有语言模型会保留。上游同步可能只返回语言模型，不能据此判断视频模型不可用。
5. 将账号加入需要提供服务的 OpenAI 分组，确认账号处于启用、可调度状态。用户的 star-X API Key 需要绑定对应分组。

上游 API Key 保存在管理员的账号配置中；用户接入始终使用自己的 star-X API Key。语言模型通过 `/v1/chat/completions` 调用，视频模型通过 `/v2/video_generation` 调用。

模型加入账号列表只是登记可路由能力，不验证上游授权、余额或生成权限。实际可用性需要使用相应账号完成调用确认。

## 管理员设置开放与定价

在「模型服务」中找到模型，点击「配置模型」。先选择本次修改的分组范围，再维护「规格与价格」「开放分组」「上游来源」。H3 和 H3-Max 分别设置，各自使用独立规格与价格。

管理员为模型设置基础售价，再为关联分组勾选开放的规格子集。分组倍率在「分组与倍率」中管理；沿用既有分组倍率或视频独立倍率。应用前会确认影响范围，修改已发布模型使用「预览并应用」，需要停止新调用时单独选择「暂停服务」。

未设置模型服务规则时继续沿用原有图片／视频分组权限及系统默认价格；设置独立视频规则后，以发布状态和分组规则决定可用范围。只应向已有可调度上游账号的分组开放模型。

站点以美元记录用量。系统默认的每秒基础售价为：

| 模型 | 分辨率 | 默认基础售价（USD／秒） |
| --- | --- | --- |
| MiniMax-H3 | 768P | 0.08 |
| MiniMax-H3 | 2K | 0.13 |
| MiniMax-H3-Max | 480P | 0.05 |
| MiniMax-H3-Max | 768P | 0.08 |

默认值参考 [MiniMax 国际站按量价格](https://platform.minimax.io/docs/guides/pricing-paygo)，管理员可覆盖为自己的对外售价。中国站上游以人民币结算，采购成本与站点美元售价可能不同。报价和结算使用同一份规格、售价与倍率快照。

## 用户如何接入与测试

用户进入「模型目录」，按文本或视频筛选，找到模型后选择「API 接入」或「在线测试」。同一个详情页共享调用分组与调用密钥；API 示例使用占位符，不展示用户密钥原文。

用户端不提供「价格与限制」面板，也没有「我的视频任务」入口。在线测试仍需确认会消耗账户额度；点击「开始测试」后，页面先校验本次配置，再提交一次生成请求。视频状态和结果显示在本次测试窗口，接入用户自己的应用时，应用通过任务编号查询。

切换分组或密钥会重置当前测试显示，已受理的任务不会取消，请保留任务编号。关闭页面后，仍可通过 API 查询。结果 URL 由上游提供，可能过期，应及时保存。

## 视频 API 调用示例

以下为 H3-Max 最短文生视频示例。先以相同请求体获取报价版本：

```bash
curl -X POST 'https://YOUR_STAR_X_HOST/v2/video_generation/quote' \
  -H 'Authorization: Bearer YOUR_STAR_X_API_KEY' \
  -H 'Content-Type: application/json' \
  -d '{"model":"MiniMax-H3-Max","content":[{"type":"text","text":"夕阳下的海岸线，缓慢推进镜头"}],"resolution":"480P","duration":5,"ratio":"16:9"}'
```

将响应中的 `version` 填入 `X-Video-Quote`，保持请求体一致，再提交任务：

```bash
curl -X POST 'https://YOUR_STAR_X_HOST/v2/video_generation' \
  -H 'Authorization: Bearer YOUR_STAR_X_API_KEY' \
  -H 'Content-Type: application/json' \
  -H 'X-Video-Quote: QUOTE_VERSION' \
  -H 'Idempotency-Key: YOUR_UNIQUE_REQUEST_ID' \
  -d '{"model":"MiniMax-H3-Max","content":[{"type":"text","text":"夕阳下的海岸线，缓慢推进镜头"}],"resolution":"480P","duration":5,"ratio":"16:9"}'
```

如需调用 H3，将模型改为 `MiniMax-H3`，分辨率改为 `768P` 或 `2K`，时长设为 4–15 秒。参数改变后需重新获取报价版本；如果带入的版本与当前规则不符，提交返回 409。

每个新任务设置独立的 `Idempotency-Key`（幂等请求编号），同一次任务的重试复用原值；不要在改变参数后复用同一个编号。若提交结果不确定，保留请求编号并联系管理员核对，避免新建重复任务。

创建响应返回 `task_id` 后，使用创建任务的 star-X API Key 查询：

```bash
curl 'https://YOUR_STAR_X_HOST/v2/query/video_generation/TASK_ID' \
  -H 'Authorization: Bearer YOUR_STAR_X_API_KEY'
```

状态为 `succeeded` 时，从 `task.content.url` 获取视频地址。任务查询不会重复计费。兼容入口 `/v1/videos/generations` 也可提交相同格式，或提交包含 `model`、`prompt`、`resolution`、`duration`、`ratio` 的简化请求。

## 任务与账务

管理员通过独立的「任务中心」入口核对视频任务与账务。服务保留任务的用户、密钥、分组、规格与价格快照，并持续查询已受理任务的状态，便于排错与账务处理。

余额付费任务提交前预占额度，上游受理后结算一次；明确未受理时释放预占。不确定提交进入管理员核对流程，不自动重新提交。仅在供应商后台确认未受理且未收费后，管理员才应释放不确定任务的预占额度。

生成失败不代表供应商免费。管理员核对后可退还余额，退款不重置调用用量限额；订阅任务通过订阅管理处理补偿。

本文描述当前适配与操作流程，不作为任何上游账号已完成真实付费生成验证的记录。模型规格以 [MiniMax 官方接口概览](https://platform.minimax.cn/docs/api-reference/api-overview) 为参考，站点实际支持范围以上文为准。
