# Findings

## 2026-04-08

- 当前 new-api 的上游签到不是参考实现：
  - UI 仍然要求手填 `upstream_checkin_user_id` 和 `upstream_checkin_access_token`
  - 数据保存在 `channel.settings` / `channel.other_info`
- 当前关键文件：
  - `/Volumes/SSD/Projects/_projects/new-api/service/channel_upstream_helper.go`
  - `/Volumes/SSD/Projects/_projects/new-api/controller/channel_upstream_helper.go`
  - `/Volumes/SSD/Projects/_projects/new-api/web/src/components/table/channels/upstream/index.jsx`
- 参考项目 `metapi` 的核心结构已经核实：
  - `accounts` 表保存上游账号/会话
  - `checkin_logs` 表保存历史签到结果
  - 登录后自动拉取 `accessToken/apiToken`
  - `new-api` 家族签到时会自动发现 `platformUserId`
- 参考证据：
  - `/tmp/metapi-cita/src/server/routes/api/accounts.ts`
  - `/tmp/metapi-cita/src/server/services/platforms/newApi.ts`
  - `/tmp/metapi-cita/src/server/services/checkinService.ts`
  - `/tmp/metapi-cita/src/server/db/schema.ts`
- 当前 upstream page 只是“channel 列表二次筛选”，并不是独立站点/账号/日志模型。
- 这次最重要的结构决策已经定下：
  - 不再继续把签到逻辑堆在 channel 上
  - 新增独立 upstream site/account/checkin log 模型
