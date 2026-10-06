# 蚌埠市住建局维修资金拨付公示爬虫

抓取 [蚌埠市住房和城乡建设局](https://zjj.bengbu.gov.cn) 住宅专项维修资金拨付公示信息，并支持 Web 页面查询。

## 功能

- 自动爬取 371 页公示列表，获取每篇文章详情
- 从正文 HTML 表格中提取金额（"拨付金额"列汇总），无表格时从文本提取，总计金额用于统计
- 正文表格自动格式化为 Markdown 风格的管道表格（`|` 分隔）
- 去重：已入库文章自动跳过
- Web 查询页面：搜索、排序（标题/金额/日期）、分页（20/50/100 条）、页码跳转、点击查看详情
- 邮件订阅：输入邮箱和小区名称，新文章发布时自动发送通知
- 敏感词过滤：订阅时对邮箱和小区名称进行敏感词校验

## 用法

```bash
go build -o crawler .

./crawler                  # 启动 Web 服务 + 爬虫循环（数据库 ./bengbu_wxjj.db，端口 20173）
./crawler -server          # 仅启动 Web 服务，永不爬取
./crawler -once            # 爬取一轮后退出（仍会先启动 Web 服务，退出时随之结束）

DB_PATH=/path/to/db ./crawler          # 指定数据库路径
PORT=9090 ./crawler -server            # 指定端口（默认 20173）
SMTP_EMAIL=xxx@qq.com SMTP_PASS=xxx ./crawler   # 覆盖内置邮箱和授权码
```

## 爬取策略

- 列表页：`https://zjj.bengbu.gov.cn/content/column/6801691?pageIndex=N`
- 首次运行（数据库为空）：全量爬取 371 页
- 已有数据后：每次只爬前 3 页，检查是否有新公示
- 每 6 小时自动重复一轮（进程内 `time.Sleep` 循环，不是 cron）
- 限速：详情页间隔 800ms、列表页间隔 1500ms；列表页失败则等 3s 后跳过，该页本轮丢失、下轮再试
- 每轮结束后执行一次订阅通知邮件

## 依赖

- [goquery](https://github.com/PuerkitoBio/goquery) — HTML 解析
- [modernc.org/sqlite](https://modernc.org/sqlite) — SQLite 驱动（纯 Go 实现，无需 CGO/gcc）

## 邮件订阅

支持用户按小区名称订阅，新公示发布时自动通过 QQ 邮箱 SMTP 发送通知。

- 邮箱和小区名称后端限制 **30 字节**（按 UTF-8 字节计，中文小区名约 10 个字）；前端输入框 `maxlength=30` 按字符计，两者不一致以后端为准
- 输入内容经过**敏感词过滤**后方可提交
- 每封邮件包含匹配小区的多条新公示，附带取消订阅链接
- 数据库记录唯一约束 `(email, community)`，重复订阅自动恢复并更新 token，同时 `last_article_id` 归零
- 邮件发送失败时不推进 `last_article_id`，下一轮（6 小时后）自动重试
- 新订阅的 `last_article_id` 从 0 开始，**首次通知会把全部历史匹配文章一并发给订阅者**

环境变量：

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `DB_PATH` | SQLite 数据库路径 | `./bengbu_wxjj.db` |
| `PORT` | Web 服务端口 | `20173` |
| `SMTP_EMAIL` | QQ 邮箱地址 | `main.go:27` 中硬编码的内置值 |
| `SMTP_PASS` | QQ 邮箱 SMTP 授权码 | `main.go:27` 中硬编码的内置值 |
| `BASE_URL` | 取消订阅链接的基础 URL | `https://wxzj.dirac.eu.org` |

> ⚠️ **已知问题**：`SMTP_EMAIL` / `SMTP_PASS` 的默认值硬编码在 `main.go:27`，该授权码已进入 git 历史。建议改为无默认值并设为必填环境变量，同时前往 QQ 邮箱后台重置该授权码。注意 `getEnv` 把空字符串视为未设置，因此 `SMTP_PASS=` 仍会回退到内置值。

## 数据库

### articles 表

| 字段 | 说明 |
|------|------|
| id | 自增主键 |
| title | 标题 |
| url | 链接（唯一） |
| publish_date | 发布日期 |
| content | 正文（已清洗） |
| amount | 金额（从 HTML 表格提取汇总） |
| created_at | 入库时间 |

### subscriptions 表

| 字段 | 说明 |
|------|------|
| id | 自增主键 |
| email | 订阅邮箱 |
| community | 小区名称 |
| unsub_token | 取消订阅唯一令牌（32 位十六进制随机串） |
| last_article_id | 已通知的最新文章 ID |
| created_at | 订阅时间 |
| unsubscribed_at | 取消订阅时间（NULL 表示活跃） |
