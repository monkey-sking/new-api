# Progress

## 2026-04-08

- 对照了实际仓库与参考项目，确认当前偏差不是“小功能没补”，而是缺了账号层。
- 写入了本次 workstream 的设计文档：
  - `docs/superpowers/specs/2026-04-08-upstream-account-checkin-design.md`
- 重置了项目根目录的 `task_plan.md` 和 `findings.md`，把焦点切到 `feature/upstream-account-checkin`。
- 新增了独立的：
  - `upstream_sites`
  - `upstream_accounts`
  - `upstream_checkin_logs`
- 新增后端能力：
  - 站点 CRUD
  - 账号 CRUD
  - 账号刷新会话
  - `new-api` 家族自动发现 `platform user id`
  - 账号级签到
  - 签到历史列表
  - 账号级定时签到任务
- 重写了 `/console/channel/upstream` 页面，现在以 `站点 / 账号 / 签到记录` 为主视角。
- 已通过：
  - upstream 定向 Go 测试
  - 前端 ESLint
  - 前端构建
  - 主程序 `go build`
- 已把新版本编进并重启 `http://127.0.0.1:3000/` 对应实例。

## 2026-04-14

- 关闭了本机 `com.runking.new-api` LaunchAgent（端口 `3000` 不再监听）。
- 清理了仓库根目录下的本地构建产物（`new-api-m`、`new-api-macos-custom`），并补充 `.gitignore` 忽略规则以避免再次出现。
