# 蚌埠市住建局维修资金拨付公示爬虫

抓取 [蚌埠市住房和城乡建设局](https://zjj.bengbu.gov.cn) 住宅专项维修资金拨付公示信息，并支持 Web 页面查询。

## 功能

- 自动爬取 371 页公示列表，获取每篇文章详情
- 从正文 HTML 表格中提取金额（"拨付金额"列汇总）
- 去重：已入库文章自动跳过
- Web 查询页面：搜索、排序（标题/日期/金额）、分页、点击查看详情

## 用法

```bash
go build -o crawler .

./crawler                  # 爬取数据（默认数据库 ./bengbu_wxjj.db）
./crawler -server          # 启动 Web 查询页面 http://localhost:8080

DB_PATH=/path/to/db ./crawler          # 指定数据库路径
PORT=9090 ./crawler -server            # 指定端口
```

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