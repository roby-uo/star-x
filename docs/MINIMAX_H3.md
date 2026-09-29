# MiniMax-H3 视频接入

## 配置上游账号

在后台创建 OpenAI 协议的 API Key 上游账号，填入 MiniMax API Key，并将 Base URL 设置为 `https://api.minimax.cn`（中国站）或 `https://api.minimax.io`（国际站）。如果后台表单要求 `/v1` 后缀，也可以填写；视频接口会使用同一主机的 `/v2` 路径。

在「模型白名单」中添加 `MiniMax-H3`；若账号已经同步了语言模型，可直接点击「添加 MiniMax-H3 视频模型」保留原有模型。也可以在「模型映射」中配置 `MiniMax-H3` → `MiniMax-H3`。上游模型同步接口可能只返回语言模型，不能据此判断 H3 不可用。

将账号加入一个 OpenAI 分组。未设置模型服务规则时继续沿用原有图片/视频权限；建议在管理端「模型服务 → 管理开放」配置 H3 的独立发布状态与分组规则。用户 API Key 需要绑定此分组。只有配置在可调度账号上的模型才会显示于用户端模型管理。这里借用 OpenAI 账号类型管理密钥和分组；H3 视频请求实际使用 MiniMax 原生 `/v2` 异步任务接口。将模型加入列表不代表上游 API Key 已开通视频权限，最终应以实际提交视频任务为准。

## 调用

使用 star-X 的 API Key 提交任务：

```bash
curl -X POST 'https://YOUR_STAR_X_HOST/v2/video_generation' \
  -H 'Authorization: Bearer YOUR_STAR_X_API_KEY' \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: YOUR_UNIQUE_REQUEST_ID' \
  -d '{"model":"MiniMax-H3","content":[{"type":"text","text":"夕阳下的海岸线，缓慢推进镜头"}],"resolution":"768P","duration":5,"ratio":"16:9"}'
```

响应包含 `task_id`。查询同一 API Key 创建的任务：

```bash
curl 'https://YOUR_STAR_X_HOST/v2/query/video_generation/TASK_ID' \
  -H 'Authorization: Bearer YOUR_STAR_X_API_KEY'
```

状态为 `succeeded` 时，从 `task.content.url` 获取视频地址。还可使用 `/v1/videos/generations` 提交相同格式，或提交简化的 `{ "model":"MiniMax-H3", "prompt":"...", "resolution":"768P", "duration":5, "ratio":"16:9" }`。

当前公开入口支持文生视频、单张首帧/尾帧及双帧图生视频；不接受参考视频/音频/参考图片。H3 输出时长为 4–15 秒，分辨率为 `768P` 或 `2K`。新建视频任务持久保存用户、API Key、分组、规格及价格快照。后台持续更新已受理任务状态，用户可在「模型管理 → 我的视频任务」中查看；任务查询不会重复计费。旧版本创建的任务仍可在原缓存有效期内查询。结果 URL 由上游提供，可能过期，请及时保存。

站点以美元记录用量，默认输出单价为 768P 每秒 $0.08、2K 每秒 $0.13，并乘以用户/分组的视频倍率。该美元价格取自 MiniMax 国际站公开 API 价目；中国站上游以人民币结算，实际成本可能不同。管理员可在模型服务中覆盖每种分辨率的每秒售价，报价与结算使用同一价格快照。余额付费任务提交前预占额度，上游受理后结算一次；明确未受理时释放预占，网络超时等不确定情况进入管理员核对流程，不自动重新提交。后续失败不假定供应商免费；管理员核对后可退还余额（不重置调用用量限额），订阅任务通过订阅管理处理补偿。


## 管理端与用户端

1. 管理端「模型服务」找到 H3，点击「管理开放」。选择已有渠道，或为尚未归属渠道的分组直接创建渠道。
2. 配置开放的分辨率、时长与输入方式，以及 USD 每秒售价。为每个关联分组勾选允许的子集，再发布。未勾选的组不能调用该模型。
3. 用户「模型管理」选择 H3 的「选规格／生成」，选择自己的密钥，输入内容并获取报价，再点击带费用的生成按钮。
4. 参数改变后旧报价自动失效。API 调用可先 POST `/v2/video_generation/quote`（请求体与原生生成相同），将响应 `version` 通过 `X-Video-Quote` 请求头随生成请求提交；版本发生变化时返回 409。
5. 每次新任务设置独立 `Idempotency-Key`，同一任务重试复用原值。变更参数后不能复用相同值。
6. 管理员在「模型服务 → 视频任务与账务」查看异常任务。仅在供应商后台确认未受理且未收费后释放不确定任务的预占额度。

模型白名单只是登记可路由的模型，不验证上游授权；代码测试不等于真实付费生成验证。当前规格编辑与报价聚焦 H3，尚未开放 H3-Max 或参考视频／音频生成。
