# 本地 launchd 部署维护说明

这份文档只保留可复用的维护流程，不记录真实用户名、家目录、密钥或机器特定路径。需要落地到某台机器时，请先把下面的占位符替换成该机器的实际值。

## 本机变量模板

- 仓库目录：`<REPO_ROOT>`
- 运行二进制：`<BIN_PATH>`
- launchd 配置：`<PLIST_PATH>`
- 服务标签：`<LAUNCHD_LABEL>`
- 监听地址：`http://127.0.0.1:<PORT>`
- 工作目录：`<WORKDIR>`
- 数据库：`<DB_PATH>`
- 日志目录：`<LOG_DIR>`
- 标准输出：`<STDOUT_LOG>`
- 标准错误：`<STDERR_LOG>`

建议在维护前先导出一组本地变量，后面的命令都按这组变量执行：

```bash
export REPO_ROOT="<REPO_ROOT>"
export BIN_PATH="<BIN_PATH>"
export PLIST_PATH="<PLIST_PATH>"
export LAUNCHD_LABEL="<LAUNCHD_LABEL>"
export PORT="<PORT>"
export DB_PATH="<DB_PATH>"
export LOG_DIR="<LOG_DIR>"
export STDOUT_LOG="<STDOUT_LOG>"
export STDERR_LOG="<STDERR_LOG>"
```

## launchd 配置要点

`plist` 建议至少包含：

- `KeepAlive = true`
- `RunAtLoad = true`
- `ProgramArguments = <BIN_PATH> --port <PORT> --log-dir <LOG_DIR>`
- 固定 `SESSION_SECRET`

注意：

- 固定 `SESSION_SECRET` 很重要，但不要把真实值写进仓库文档。
- 没有固定 `SESSION_SECRET` 时，服务每次重启都可能让后台登录态失效，页面会跳到 `/login?expired=true`。

## 日常检查

查看服务状态：

```bash
launchctl print "gui/$(id -u)/$LAUNCHD_LABEL" | sed -n '1,120p'
```

健康检查：

```bash
curl -sS "http://127.0.0.1:$PORT/api/status"
```

看实时日志：

```bash
tail -f "$STDOUT_LOG"
tail -f "$STDERR_LOG"
```

查看端口占用：

```bash
lsof -nP -iTCP:"$PORT" -sTCP:LISTEN
```

## 从源码更新本地运行实例

先在仓库目录完成代码检查和测试：

```bash
cd "$REPO_ROOT"
go test ./middleware ./model ./relay/...
```

构建新二进制：

```bash
cd "$REPO_ROOT"
go build -o new-api-macos-custom ./
```

替换运行中的二进制前先备份：

```bash
cp "$BIN_PATH" \
  "$BIN_PATH.backup-$(date +%Y%m%d-%H%M%S)"
```

部署新二进制：

```bash
cp "$REPO_ROOT/new-api-macos-custom" "$BIN_PATH"
```

重启服务：

```bash
launchctl kickstart -k "gui/$(id -u)/$LAUNCHD_LABEL"
```

重启后立即验证：

```bash
curl -sS "http://127.0.0.1:$PORT/api/status"
lsof -nP -iTCP:"$PORT" -sTCP:LISTEN
tail -n 100 "$STDOUT_LOG"
tail -n 100 "$STDERR_LOG"
```

## 回滚

如果新版本启动异常，直接回滚到最近备份：

```bash
cp "$BIN_PATH.backup-YYYYMMDD-HHMMSS" "$BIN_PATH"
launchctl kickstart -k "gui/$(id -u)/$LAUNCHD_LABEL"
```

然后重新做一次健康检查和日志检查。

## Codex 本地中转配置

本机 Codex App 走兼容模式时，应使用：

- Base URL：`http://127.0.0.1:<PORT>/v1`
- API Key：后台真实生成的 `sk-...` 令牌

不要把 Base URL 误填到 API Key。之前出现过以下错误场景：

- TCP 已成功到达 `127.0.0.1:<PORT>`
- 但请求头变成了 `Authorization: Bearer http://127.0.0.1:<PORT>/v1`
- 网关日志表现为 `TokenAuth: auth failed for key starting with [http]`

如果 Codex 侧提示 `builder error`，先确认是不是这个问题。

## Codex 排障建议

先查最近是否有 `codex` 令牌记录：

```bash
sqlite3 "$DB_PATH" \
  "select datetime(created_at,'unixepoch','localtime'), channel_id, model_name, token_name, type, quota from logs where token_name='codex' order by created_at desc limit 20;"
```

如果没有新记录，优先看认证失败日志：

```bash
rg "TokenAuth" "$STDERR_LOG"
```

如果有新记录，再看请求和流总结：

```bash
rg "responses debug request|responses debug stream summary|record consume log" \
  "$STDOUT_LOG"
```

当前仓库里已经有两类针对 Codex 的临时调试信息：

- `responses debug request`
- `responses debug stream summary`

它们用于判断：

- 请求是否真的进了本地 `/v1/responses`
- 网关是否看到了 `response.completed`
- 成功流的事件类型序列大致是什么

## 已知现象

- `GET /v1/responses` 返回 `404` 不代表主流程出错，Codex 可能会有探测请求。
- `openclaw` 和 `codex` 是两套不同流量，排查 Codex 时不要混看。
- 某些机器可能是 launchd 常驻部署，不是仓库里 `go run main.go` 的开发启动方式。

## 不建议提交的本地产物

仓库根目录可能会出现以下本地产物：

- `new-api-m`
- `new-api-macos-custom`

它们只是本机构建结果，不应该作为源码变更提交。
