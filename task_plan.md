# Task Plan

## Goal
把当前“渠道里手填 token/user_id 的上游签到”升级成参考项目那种“独立上游站点/账号/签到日志 + 自动获取会话与用户 ID”的结构。

## Workstream
- Name: `feature/upstream-account-checkin`
- Risk: High
- Validation target: targeted Go tests + frontend build + source review

## Baseline
- Base path: `/Volumes/SSD/Projects/_projects/new-api`
- Reference repo snapshot: `/tmp/metapi-cita`
- Backend owner surfaces:
  - `model/*`
  - `service/channel_upstream_helper.go` and new upstream service files
  - `controller/channel_upstream_helper.go` and new upstream controllers
  - `router/api-router.go`
- Frontend owner surfaces:
  - `web/src/pages/ChannelUpstream/*`
  - `web/src/components/table/channels/upstream/*`
  - `web/src/App.jsx`
  - `web/src/components/layout/SiderBar.jsx`
- Out of scope:
  - OAuth provider parity
  - replacing channel relay data model
  - packaging/release work

## Intake Baseline
- Target user: 单管理员、自用的 new-api 运营者
- Desired user-visible outcome:
  - 不再手填 `user_id`
  - 能像参考项目一样用上游账号登录自动拿会话
  - 在独立页面里看站点、账号和签到历史
- Scope exclusions:
  - 不做整套 OAuth 管理
  - 不把 channels 全量迁到 sites/accounts
- Acceptance criteria:
  - 新增独立 `upstream site / account / checkin log` 数据表
  - 支持登录上游账号后自动拿 `access token`
  - 对 `new-api` 家族站点可自动发现 `platform user id`
  - 立即签到与定时签到都按账号执行
  - `/console/channel/upstream` 以站点/账号/日志为主视角

## Phases
- [x] 对照参考项目，确认当前实现缺的是账号层而不是输入框自动填充
- [x] 写设计基线并记录到仓库
- [ ] 新增 upstream site/account/checkin log 模型并接入 AutoMigrate
- [ ] 实现上游账号登录、token 导入、user id 自动发现、API token 拉取
- [ ] 实现账号级签到执行与日志写入
- [ ] 改造 `/console/channel/upstream` 为 site/account/log 视角
- [ ] 做 targeted tests/build 并同步本地运行环境

## Current Findings
- 当前实现把签到状态写在 `channel.other_info`，把签到输入写在 `channel.settings`
- 参考项目的核心链路是 `site -> account -> checkin log`
- 参考项目真正有用的能力是：
  - 登录拿 session/access token
  - 自动发现 `platformUserId`
  - 账号级签到
  - 历史日志

## Risks
- 仓库存在 unrelated dirty files，必须避免误改
- 这次是结构升级，不是小修，接口和页面都要一起换
- 某些上游面板兼容差异可能要后续补适配
