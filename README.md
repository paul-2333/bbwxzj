# 蚌埠市住建局维修资金拨付公示爬虫

抓取 [蚌埠市住房和城乡建设局](https://zjj.bengbu.gov.cn) 住宅专项维修资金拨付公示信息，并支持 Web 页面查询。

## 功能

- 自动爬取 371 页公示列表，获取每篇文章详情
- 从正文 HTML 表格中提取金额（"拨付金额"列汇总），无表格时从文本提取
- 去重：已入库文章自动跳过
- Web 查询页面：搜索、排序（标题/金额/日期）、分页（20/50/100 条）、页码跳转、点击查看详情

## 用法

```bash
go build -o crawler .

./crawler                  # 启动 Web 服务 + 后台爬取（默认数据库 ./bengbu_wxjj.db）
./crawler -server          # 仅启动 Web 服务（不爬取）
./crawler -once            # 仅爬取一次（不启动 Web）

DB_PATH=/path/to/db ./crawler          # 指定数据库路径
PORT=9090 ./crawler -server            # 指定端口
```

## 爬取策略

- 首次运行：全量爬取 371 页
- 已有数据后：每次只爬前 3 页，检查是否有新公示
- 每6小时自动重复一轮

## 依赖

- [goquery](https://github.com/PuerkitoBio/goquery) — HTML 解析
- [go-sqlite3](https://github.com/mattn/go-sqlite3) — SQLite 驱动（CGO，需要 gcc）

## 数据库

| 字段 | 说明 |
|------|------|
| id | 自增主键 |
| title | 标题 |
| url | 链接（唯一） |
| publish_date | 发布日期 |
| content | 正文（已清洗） |
| amount | 金额（从 HTML 表格提取汇总） |
| created_at | 入库时间 |