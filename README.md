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

./crawler                  # 启动 Web 服务 + 后台爬取（默认数据库 ./bengbu_wxjj.db）
./crawler -server          # 仅启动 Web 服务（不爬取）
./crawler -once            # 仅爬取一次（不启动 Web）

DB_PATH=/path/to/db ./crawler          # 指定数据库路径
PORT=9090 ./crawler -server            # 指定端口
SMTP_EMAIL=xxx@qq.com SMTP_PASS=xxx ./crawler   # 自定义邮箱和授权码
```

## 爬取策略

- 首次运行：全量爬取 371 页
- 已有数据后：每次只爬前 3 页，检查是否有新公示
- 每6小时自动重复一轮

## 依赖

- [goquery](https://github.com/PuerkitoBio/goquery) — HTML 解析
- [go-sqlite3](https://github.com/mattn/go-sqlite3) — SQLite 驱动（CGO，需要 gcc）

## 邮件订阅

支持用户按小区名称订阅，新公示发布时自动通过 QQ 邮箱 SMTP 发送通知。

- 邮箱和小区名称均限制 **30 个字符**
- 输入内容经过**敏感词过滤**后方可提交
- 每封邮件包含匹配小区的多条新公示，附带取消订阅链接
- 数据库记录唯一约束 `(email, community)`，重复订阅自动恢复并更新 token

环境变量：

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `SMTP_EMAIL` | QQ 邮箱地址 | `1098703551@qq.com` |
| `SMTP_PASS` | QQ 邮箱 SMTP 授权码 | `hrhllcunoioggaej` |
| `BASE_URL` | 取消订阅链接的基础 URL | `https://wxzj.dirac.eu.org` |

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
| unsub_token | 取消订阅唯一令牌（UUID） |
| last_article_id | 已通知的最新文章 ID |
| created_at | 订阅时间 |
| unsubscribed_at | 取消订阅时间（NULL 表示活跃） |