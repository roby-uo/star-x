# MiniMax-H3 视频接入

## 配置上游账号

在后台创建 OpenAI 协议的 API Key 上游账号，填入 MiniMax API Key，并将 Base URL 设置为 `https://api.minimax.cn`（中国站）或 `https://api.minimax.io`（国际站）。如果后台表单要求 `/v1` 后缀，也可以填写；视频接口会使用同一主机的 `/v2` 路径。

将账号的模型映射配置为 `MiniMax-H3` → `MiniMax-H3`，加入一个 OpenAI 分组，并开启该分组的图片/视频生成权限。用户 API Key 需要绑定此分组。只有配置在可调度账号上的模型才会显示于用户端模型管理。

## 调用

使用 star-X 的 API Key 提交任务：

```bash
curl -X POST 'https://YOUR_STAR_X_HOST/v2/video_generation' \
  -H 'Authorization: Bearer YOUR_STAR_X_API_KEY' \
  -H 'Content-Type: application/json' \
  -d '{"model":"MiniMax-H3","content":[{"type":"text","text":"夕阳下的海岸线，缓慢推进镜头"}],"resolution":"768P","duration":5,"ratio":"16:9"}'
```

响应包含 `task_id`。查询同一 API Key 创建的任务：

```bash
curl 'https://YOUR_STAR_X_HOST/v2/query/video_generation/TASK_ID' \
  -H 'Authorization: Bearer YOUR_STAR_X_API_KEY'
```

状态为 `succeeded` 时，从 `task.content.url` 获取视频地址。还可使用 `/v1/videos/generations` 提交相同格式，或提交简化的 `{ "model":"MiniMax-H3", "prompt":"...", "resolution":"768P", "duration":5, "ratio":"16:9" }`。

当前公开入口支持文生视频、单张首帧/尾帧及双帧图生视频；不接受参考视频/音频/参考图片。H3 输出时长为 4–15 秒，分辨率为 `768P` 或 `2K`。视频任务绑定发起用户、API Key 和分组，最多保存 7 天；任务查询不会重复计费。

站点以美元记录用量，默认输出单价为 768P 每秒 $0.08、2K 每秒 $0.13，并乘以用户/分组的视频倍率。该美元价格取自 MiniMax 国际站公开 API 价目；中国站上游以人民币结算，实际成本可能不同。创建任务被上游接受后记录一次用量，任务后续失败时目前不会自动退款。
