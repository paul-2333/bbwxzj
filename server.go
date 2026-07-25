package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	_ "github.com/mattn/go-sqlite3"
)

type ArticleRow struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	PublishDate string `json:"publishDate"`
	Amount      string `json:"amount"`
	Content     string `json:"content"`
	URL         string `json:"url"`
	CreatedAt   string `json:"createdAt"`
}

type Stats struct {
	Total       int     `json:"total"`
	WithAmount  int     `json:"withAmount"`
	TotalAmount float64 `json:"totalAmount"`
}

type PageData struct {
	Stats    Stats
	Articles []ArticleRow
	Total    int
	Page     int
	PerPage  int
	Query    string
	Sort     string
	Order    string
}

type APIResponse struct {
	Articles []ArticleRow `json:"articles"`
	Total    int          `json:"total"`
	Page     int          `json:"page"`
	PerPage  int          `json:"perPage"`
	Stats    Stats        `json:"stats"`
}

const pageHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>蚌埠市住建局 - 维修资金拨付公示查询</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,"Helvetica Neue",Arial,"Noto Sans SC",sans-serif;background:#f0f2f5;color:#333;min-height:100vh}
.header{background:linear-gradient(135deg,#1a73e8,#1557b0);color:#fff;padding:28px 0 24px;box-shadow:0 2px 8px rgba(0,0,0,.15)}
.header h1{font-size:24px;font-weight:600;letter-spacing:1px}
.header p{font-size:13px;opacity:.85;margin-top:4px}
.container{max-width:1400px;margin:0 auto;padding:0 20px}
.stats{display:grid;grid-template-columns:repeat(auto-fit,minmax(200px,1fr));gap:16px;margin:-20px auto 24px;max-width:1400px;padding:0 20px;position:relative;z-index:1}
.stat-card{background:#fff;border-radius:10px;padding:18px 22px;box-shadow:0 1px 4px rgba(0,0,0,.08);text-align:center}
.stat-card .label{font-size:13px;color:#888;margin-bottom:4px}
.stat-card .value{font-size:28px;font-weight:700;color:#1a73e8}
.stat-card .value.green{color:#0d904f}
.stat-card .value.orange{color:#e67e22}
.toolbar{background:#fff;border-radius:10px;padding:16px 20px;margin-bottom:20px;box-shadow:0 1px 4px rgba(0,0,0,.08);display:flex;flex-wrap:wrap;gap:12px;align-items:center}
.toolbar input,.toolbar select{padding:8px 14px;border:1px solid #d0d5dd;border-radius:6px;font-size:14px;outline:none;transition:border-color .2s}
.toolbar input:focus,.toolbar select:focus{border-color:#1a73e8}
.toolbar input{flex:1;min-width:200px}
.toolbar .btn{padding:8px 20px;background:#1a73e8;color:#fff;border:none;border-radius:6px;font-size:14px;cursor:pointer;transition:background .2s}
.toolbar .btn:hover{background:#1557b0}
.table-wrap{background:#fff;border-radius:10px;overflow:hidden;box-shadow:0 1px 4px rgba(0,0,0,.08);margin-bottom:20px;overflow-x:auto}
table{width:100%;border-collapse:collapse;font-size:14px}
thead{background:#f8f9fa}
th{padding:12px 14px;text-align:left;font-weight:600;color:#555;white-space:nowrap;cursor:pointer;user-select:none;position:relative}
th:hover{background:#eef1f5}
th .sort-arrow{font-size:10px;margin-left:4px;color:#999}
td{padding:10px 14px;border-top:1px solid #eee;max-width:400px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
tr:hover{background:#f5f8ff}
tr:nth-child(even){background:#fafbfc}
tr:nth-child(even):hover{background:#f5f8ff}
.amount{font-weight:600;color:#0d904f;white-space:nowrap}
.amount.empty{color:#bbb;font-weight:400}
.date{color:#888;white-space:nowrap;font-size:13px}
.title-link{color:#1a73e8;text-decoration:none}
.title-link:hover{text-decoration:underline}
.pagination{display:flex;justify-content:center;align-items:center;gap:8px;padding:16px 0 32px}
.pagination a,.pagination span{padding:7px 14px;border-radius:6px;font-size:14px;text-decoration:none;color:#333;transition:all .2s}
.pagination a{background:#fff;border:1px solid #d0d5dd}
.pagination a:hover{background:#eef1f5}
.pagination .active{background:#1a73e8;color:#fff;border-color:#1a73e8}
.pagination .disabled{color:#bbb;cursor:default}
.pagination .info{color:#888;font-size:13px;margin:0 8px}
.modal-overlay{display:none;position:fixed;top:0;left:0;right:0;bottom:0;background:rgba(0,0,0,.45);z-index:1000;justify-content:center;align-items:center}
.modal-overlay.active{display:flex}
.modal{background:#fff;border-radius:12px;max-width:800px;width:90%;max-height:80vh;overflow-y:auto;box-shadow:0 8px 32px rgba(0,0,0,.2);animation:modalIn .25s ease}
@keyframes modalIn{from{transform:translateY(20px);opacity:0}to{transform:translateY(0);opacity:1}}
.modal-header{padding:18px 24px;border-bottom:1px solid #eee;display:flex;justify-content:space-between;align-items:flex-start;gap:12px}
.modal-header h2{font-size:18px;line-height:1.4;flex:1}
.modal-close{background:none;border:none;font-size:24px;cursor:pointer;color:#999;padding:0 4px;line-height:1}
.modal-close:hover{color:#333}
.modal-meta{padding:12px 24px;background:#f8f9fa;font-size:13px;color:#888;display:flex;gap:20px;flex-wrap:wrap}
.modal-body{padding:20px 24px;font-size:15px;line-height:1.8;white-space:pre-wrap;word-break:break-all}
.modal-body p{margin-bottom:12px}
.footer{text-align:center;padding:20px;color:#aaa;font-size:13px}
@media(max-width:768px){
  .stats{grid-template-columns:repeat(2,1fr)}
  .toolbar{flex-direction:column}
  .toolbar input{min-width:auto;width:100%}
  th,td{font-size:13px;padding:8px 10px}
}
</style>
</head>
<body>

<div class="header">
  <div class="container">
    <h1>蚌埠市住建局 · 维修资金拨付公示</h1>
    <p>查询住宅专项维修资金拨付公示信息</p>
  </div>
</div>

<div class="stats" id="statsBar"></div>

<div class="container">
  <div class="toolbar">
    <input type="text" id="searchInput" placeholder="搜索标题、小区名、楼栋..." value="{{.Query}}">
    <select id="sortSelect">
      <option value="publish_date" {{if eq .Sort "publish_date"}}selected{{end}}>发布日期</option>
      <option value="amount" {{if eq .Sort "amount"}}selected{{end}}>金额</option>
      <option value="title" {{if eq .Sort "title"}}selected{{end}}>标题</option>
    </select>
    <select id="orderSelect">
      <option value="desc" {{if eq .Order "desc"}}selected{{end}}>降序</option>
      <option value="asc" {{if eq .Order "asc"}}selected{{end}}>升序</option>
    </select>
    <button class="btn" onclick="search()">查询</button>
  </div>

  <div class="table-wrap">
    <table>
      <thead>
        <tr>
          <th onclick="sortBy('title')">标题 <span class="sort-arrow">{{if eq .Sort "title"}}{{if eq .Order "asc"}}▲{{else}}▼{{end}}{{else}}▽{{end}}</span></th>
          <th onclick="sortBy('amount')">金额 <span class="sort-arrow">{{if eq .Sort "amount"}}{{if eq .Order "asc"}}▲{{else}}▼{{end}}{{else}}▽{{end}}</span></th>
          <th onclick="sortBy('publish_date')">日期 <span class="sort-arrow">{{if eq .Sort "publish_date"}}{{if eq .Order "asc"}}▲{{else}}▼{{end}}{{else}}▽{{end}}</span></th>
        </tr>
      </thead>
      <tbody id="tableBody"></tbody>
    </table>
  </div>

  <div class="pagination">
    <span style="display:flex;align-items:center;gap:8px;font-size:13px;color:#888;margin-right:16px">
      <select id="perPageSelect">
        <option value="20" selected>20条</option>
        <option value="50">50条</option>
        <option value="100">100条</option>
      </select>
    </span>
    <span id="pageButtons"></span>
    <span style="display:flex;align-items:center;gap:6px;font-size:13px;color:#888;margin-left:16px">
      页码 <input type="number" id="pageInput" min="1" value="1" style="width:60px;padding:6px 8px;border:1px solid #d0d5dd;border-radius:6px;outline:none">
      <button class="btn" style="padding:6px 12px;font-size:13px" onclick="goPage(parseInt(document.getElementById('pageInput').value)||1)">跳转</button>
    </span>
  </div>
</div>

<div class="modal-overlay" id="modal" onclick="closeModal(event)">
  <div class="modal" onclick="event.stopPropagation()">
    <div class="modal-header">
      <h2 id="modalTitle"></h2>
      <button class="modal-close" onclick="closeModal()">&times;</button>
    </div>
    <div class="modal-meta">
      <span id="modalDate"></span>
      <span id="modalAmount"></span>
    </div>
    <div class="modal-body" id="modalBody"></div>
  </div>
</div>

<div class="footer">数据来源：蚌埠市住房和城乡建设局 zjj.bengbu.gov.cn</div>

<script>
let currentPage = {{.Page}};
let currentQuery = '{{.Query}}';
let currentSort = '{{.Sort}}';
let currentOrder = '{{.Order}}';
let perPage = 20;

function search() {
  currentQuery = document.getElementById('searchInput').value.trim();
  perPage = parseInt(document.getElementById('perPageSelect').value) || 20;
  currentPage = 1;
  loadData();
}

function sortBy(field) {
  if (currentSort === field) {
    currentOrder = currentOrder === 'asc' ? 'desc' : 'asc';
  } else {
    currentSort = field;
    currentOrder = 'desc';
  }
  document.getElementById('sortSelect').value = currentSort;
  document.getElementById('orderSelect').value = currentOrder;
  loadData();
}

function goPage(p) {
  currentPage = p;
  loadData();
}

function loadData() {
  const params = new URLSearchParams({
    q: currentQuery,
    page: currentPage,
    perPage: perPage,
    sort: currentSort,
    order: currentOrder
  });
  fetch('/api/articles?' + params.toString())
    .then(r => r.json())
    .then(d => render(d))
    .catch(e => console.error(e));
}

function render(d) {
  renderStats(d.stats);
  renderTable(d.articles);
  renderPagination(d.total, d.page, d.perPage);
}

function renderStats(s) {
  document.getElementById('statsBar').innerHTML = [
    '<div class="stat-card"><div class="label">公示总数</div><div class="value">' + s.total + '</div></div>',
    '<div class="stat-card"><div class="label">含金额记录</div><div class="value green">' + s.withAmount + '</div></div>',
    '<div class="stat-card"><div class="label">总金额（元）</div><div class="value orange">' + s.totalAmount.toLocaleString() + '</div></div>'
  ].join('');
}

function renderTable(articles) {
  const tbody = document.getElementById('tableBody');
  if (articles.length === 0) {
    tbody.innerHTML = '<tr><td colspan="3" style="text-align:center;padding:40px;color:#999">暂无数据</td></tr>';
    return;
  }
  tbody.innerHTML = articles.map(a => {
    const amt = a.amount ? '<span class="amount">' + Number(a.amount).toLocaleString() + ' 元</span>' : '<span class="amount empty">-</span>';
    return '<tr>' +
      '<td><a class="title-link" href="javascript:void(0)" onclick="showDetail(' + a.id + ')">' + esc(a.title) + '</a></td>' +
      '<td>' + amt + '</td>' +
      '<td class="date">' + esc(a.publishDate) + '</td>' +
    '</tr>';
  }).join('');
}

function renderPagination(total, page, perPage) {
  const totalPages = Math.ceil(total / perPage);
  const el = document.getElementById('pageButtons');
  if (totalPages <= 1) { el.innerHTML = ''; return; }
  let html = '';
  html += page > 1 ? '<a href="javascript:goPage(' + (page-1) + ')">上一页</a>' : '<span class="disabled">上一页</span>';
  const range = 2;
  for (let i = Math.max(1, page - range); i <= Math.min(totalPages, page + range); i++) {
    html += i === page ? '<span class="active">' + i + '</span>' : '<a href="javascript:goPage(' + i + ')">' + i + '</a>';
  }
  html += page < totalPages ? '<a href="javascript:goPage(' + (page+1) + ')">下一页</a>' : '<span class="disabled">下一页</span>';
  html += '<span class="info">共 ' + total + ' 条</span>';
  el.innerHTML = html;
}

let articlesCache = {};

function showDetail(id) {
  if (articlesCache[id]) {
    fillModal(articlesCache[id]);
    return;
  }
  fetch('/api/article?id=' + id)
    .then(r => r.json())
    .then(a => {
      articlesCache[id] = a;
      fillModal(a);
    });
}

function fillModal(a) {
  document.getElementById('modalTitle').textContent = a.title;
  document.getElementById('modalDate').textContent = '📅 ' + (a.publishDate || '未知');
  document.getElementById('modalAmount').textContent = '💰 ' + (a.amount ? Number(a.amount).toLocaleString() + ' 元' : '无金额');
  document.getElementById('modalBody').textContent = a.content || '（暂无正文内容）';
  var link = document.createElement('p');
  link.style.marginTop = '16px';
  link.innerHTML = '<a href="' + esc(a.url) + '" target="_blank" style="color:#1a73e8">查看原文 →</a>';
  document.getElementById('modalBody').appendChild(link);
  document.getElementById('modal').classList.add('active');
}

function closeModal(e) {
  if (!e || e.target === document.getElementById('modal')) {
    document.getElementById('modal').classList.remove('active');
  }
}

function esc(s) {
  if (!s) return '';
  return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

document.addEventListener('DOMContentLoaded', function() {
  document.getElementById('searchInput').addEventListener('keydown', function(e) {
    if (e.key === 'Enter') search();
  });
  document.getElementById('sortSelect').addEventListener('change', function() {
    currentSort = this.value; loadData();
  });
  document.getElementById('orderSelect').addEventListener('change', function() {
    currentOrder = this.value; loadData();
  });
  document.getElementById('perPageSelect').addEventListener('change', function() {
    perPage = parseInt(this.value) || 50; currentPage = 1; loadData();
  });
  document.getElementById('pageInput').addEventListener('keydown', function(e) {
    if (e.key === 'Enter') goPage(parseInt(this.value) || 1);
  });
  loadData();
});
</script>
</body>
</html>`

func startServer(db *sql.DB) {
	port := getEnv("PORT", "8080")

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		tmpl, err := template.New("page").Parse(pageHTML)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		tmpl.Execute(w, PageData{
			Page:    1,
			PerPage: 20,
			Sort:    "publish_date",
			Order:   "desc",
		})
	})

	mux.HandleFunc("/api/articles", func(w http.ResponseWriter, r *http.Request) {
		handleArticlesAPI(db, w, r)
	})

	mux.HandleFunc("/api/article", func(w http.ResponseWriter, r *http.Request) {
		handleArticleDetail(db, w, r)
	})

	log.Printf("启动服务器 http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func handleArticlesAPI(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	q := r.URL.Query()

	query := q.Get("q")
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(q.Get("perPage"))
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	sort := q.Get("sort")
	order := q.Get("order")

	allowedSort := map[string]bool{"publish_date": true, "amount": true, "title": true}
	if !allowedSort[sort] {
		sort = "publish_date"
	}
	if order != "asc" && order != "desc" {
		order = "desc"
	}

	where := ""
	args := []interface{}{}
	if query != "" {
		where = "WHERE title LIKE ? OR content LIKE ?"
		like := "%" + query + "%"
		args = append(args, like, like)
	}

	sortCol := sort
	if sort == "amount" {
		sortCol = "CAST(amount AS REAL)"
	}

	var stats Stats
	countQuery := "SELECT COUNT(*) FROM articles " + where
	db.QueryRow(countQuery, args...).Scan(&stats.Total)
	db.QueryRow("SELECT COUNT(*) FROM articles WHERE amount != ''").Scan(&stats.WithAmount)
	db.QueryRow("SELECT COALESCE(SUM(CAST(amount AS REAL)), 0) FROM articles WHERE amount != ''").Scan(&stats.TotalAmount)

	offset := (page - 1) * perPage
	dataQuery := fmt.Sprintf("SELECT id, title, publish_date, amount, content, url FROM articles %s ORDER BY %s %s LIMIT ? OFFSET ?", where, sortCol, order)
	dataArgs := append(args, perPage, offset)

	rows, err := db.Query(dataQuery, dataArgs...)
	if err != nil {
		json.NewEncoder(w).Encode(APIResponse{Stats: stats})
		return
	}
	defer rows.Close()

	var articles []ArticleRow
	for rows.Next() {
		var a ArticleRow
		var amt, content, url sql.NullString
		rows.Scan(&a.ID, &a.Title, &a.PublishDate, &amt, &content, &url)
		a.Amount = amt.String
		a.Content = content.String
		a.URL = url.String
		articles = append(articles, a)
	}
	if articles == nil {
		articles = []ArticleRow{}
	}

	json.NewEncoder(w).Encode(APIResponse{
		Articles: articles,
		Total:    stats.Total,
		Page:     page,
		PerPage:  perPage,
		Stats:    stats,
	})
}

func handleArticleDetail(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid id"})
		return
	}

	var a ArticleRow
	var amt, content, url, createdAt sql.NullString
	err = db.QueryRow("SELECT id, title, publish_date, amount, content, url, created_at FROM articles WHERE id = ?", id).
		Scan(&a.ID, &a.Title, &a.PublishDate, &amt, &content, &url, &createdAt)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		return
	}
	a.Amount = amt.String
	a.Content = content.String
	a.URL = url.String
	a.CreatedAt = createdAt.String

	json.NewEncoder(w).Encode(a)
}