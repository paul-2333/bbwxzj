package main

import (
	"crypto/rand"
	"crypto/tls"
	"database/sql"
	"encoding/hex"
	"flag"
	"fmt"
	"html"
	"io"
	"log"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	_ "github.com/mattn/go-sqlite3"
)

var (
	smtpEmail = getEnv("SMTP_EMAIL", "1098703551@qq.com")
	smtpPass  = getEnv("SMTP_PASS", "hrhllcunoioggaej")
	baseURL   = getEnv("BASE_URL", "http://localhost:8080")
)

type Article struct {
	Title   string
	URL     string
	Date    string
	Content string
	Amount  string
}

func main() {
	serverMode := flag.Bool("server", false, "启动 Web 查询服务")
	onceMode := flag.Bool("once", false, "单次爬取后退出")
	flag.Parse()

	dbPath := getEnv("DB_PATH", "./bengbu_wxjj.db")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	createTable(db)
	createSubscriptionsTable(db)

	if *serverMode {
		startServer(db)
		return
	}

	go startServer(db)

	for {
		crawlAll(db)
		if *onceMode {
			break
		}
		log.Println("等待 6 小时后重新爬取...")
		time.Sleep(6 * time.Hour)
	}
}

func crawlAll(db *sql.DB) {

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			IdleConnTimeout:     30 * time.Second,
			DisableCompression:  false,
			DisableKeepAlives:   false,
			MaxIdleConnsPerHost: 5,
		},
	}

	var count int
	db.QueryRow("SELECT COUNT(*) FROM articles").Scan(&count)
	totalPages := 371
	if count > 0 {
		totalPages = 3
	}

	log.Printf("Starting crawler: %d pages (existing articles: %d)", totalPages, count)

	for page := 1; page <= totalPages; page++ {
		log.Printf("Fetching page %d/%d", page, totalPages)
		articles, err := fetchListPage(client, page)
		if err != nil {
			log.Printf("Error fetching page %d: %v", page, err)
			time.Sleep(3 * time.Second)
			continue
		}

		log.Printf("  Found %d articles on page %d", len(articles), page)
		for i, article := range articles {
			exists := articleExists(db, article.URL)
			if exists {
				log.Printf("  [%d/%d] SKIP (exists): %s", i+1, len(articles), article.Title)
				continue
			}

			log.Printf("  [%d/%d] Fetching: %s", i+1, len(articles), article.Title)
			content, amount, err := fetchDetailPage(client, article.URL)
			if err != nil {
				log.Printf("  Error fetching detail: %v", err)
				article.Content = ""
			} else {
				article.Content = content
				article.Amount = amount
			}
			insertArticle(db, article)
			time.Sleep(800 * time.Millisecond)
		}
		time.Sleep(1500 * time.Millisecond)
	}
	log.Println("Crawler completed")
	notifySubscribers(db)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func createTable(db *sql.DB) {
	query := `CREATE TABLE IF NOT EXISTS articles (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		url TEXT NOT NULL UNIQUE,
		publish_date TEXT,
		content TEXT,
		amount TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`
	_, err := db.Exec(query)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
}

func articleExists(db *sql.DB, url string) bool {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM articles WHERE url = ?", url).Scan(&count)
	return count > 0
}

func fetchListPage(client *http.Client, page int) ([]Article, error) {
	url := fmt.Sprintf("https://zjj.bengbu.gov.cn/content/column/6801691?pageIndex=%d", page)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var articles []Article
	doc.Find("ul.doc_list li").Each(func(i int, s *goquery.Selection) {
		link := s.Find("a")
		title := strings.TrimSpace(link.Text())
		href, exists := link.Attr("href")
		if !exists || href == "" {
			return
		}
		if !strings.HasPrefix(href, "http") {
			href = "https://zjj.bengbu.gov.cn" + href
		}
		date := strings.TrimSpace(s.Find("span.date").Text())
		articles = append(articles, Article{
			Title: title,
			URL:   href,
			Date:  date,
		})
	})
	return articles, nil
}

func fetchDetailPage(client *http.Client, url string) (string, string, error) {
	resp, err := client.Get(url)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return "", "", err
	}

	selectors := []string{
		"div.j-fontContent.newscontnet",
		"div.TRS_Editor",
		"div.content",
		"div.detail-content",
		"div.article-content",
		"div.news-content",
		"div#zoom",
		"div.rightnr .listnews",
	}

	var content string
	for _, sel := range selectors {
		text := doc.Find(sel).Text()
		text = strings.TrimSpace(text)
		if text != "" && len(text) > 10 {
			content = text
			break
		}
	}

	if content == "" {
		content = doc.Find("div.rightnr").Text()
		content = strings.TrimSpace(content)
	}

	amount := extractAmountFromHTML(doc)
	return cleanContent(content), amount, nil
}

func cleanContent(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	var raw []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			raw = append(raw, line)
		}
	}
	var result []string
	i := 0
	for i < len(raw) {
		if raw[i] == "序号" && i+4 < len(raw) &&
			raw[i+1] == "房号" &&
			raw[i+2] == "维修内容" &&
			raw[i+3] == "拨付金额" &&
			raw[i+4] == "备注" {
			result = append(result, "序号 | 房号 | 维修内容 | 拨付金额 | 备注")
			i += 5
			for i < len(raw) {
				if strings.HasPrefix(raw[i], "公示时间") {
					break
				}
				row := []string{raw[i]}
				i++
				for j := 0; j < 3 && i < len(raw); j++ {
					if strings.HasPrefix(raw[i], "公示时间") {
						break
					}
					row = append(row, raw[i])
					i++
				}
				if len(row) >= 4 {
					result = append(result, strings.Join(row, " | "))
				} else {
					for _, r := range row {
						result = append(result, r)
					}
				}
			}
		} else if raw[i] == "序号" && i+3 < len(raw) &&
			raw[i+1] == "房号" &&
			raw[i+2] == "维修内容" &&
			raw[i+3] == "拨付金额" {
			result = append(result, "序号 | 房号 | 维修内容 | 拨付金额")
			i += 4
			for i < len(raw) {
				if strings.HasPrefix(raw[i], "公示时间") {
					break
				}
				row := []string{raw[i]}
				i++
				for j := 0; j < 2 && i < len(raw); j++ {
					if strings.HasPrefix(raw[i], "公示时间") {
						break
					}
					row = append(row, raw[i])
					i++
				}
				if len(row) >= 3 {
					result = append(result, strings.Join(row, " | "))
				} else {
					for _, r := range row {
						result = append(result, r)
					}
				}
			}
		} else {
			result = append(result, raw[i])
			i++
		}
	}
	return strings.Join(result, "\n")
}

func extractAmountFromHTML(doc *goquery.Document) string {
	var total float64
	doc.Find("table").Each(func(_ int, table *goquery.Selection) {
		amountCol := -1
		table.Find("tr").First().Find("th, td").Each(func(i int, cell *goquery.Selection) {
			if strings.Contains(cell.Text(), "拨付金额") {
				amountCol = i
			}
		})
		if amountCol < 0 {
			return
		}
		table.Find("tr").Each(func(i int, row *goquery.Selection) {
			if i == 0 {
				return
			}
			cells := row.Find("td")
			if cells.Length() <= amountCol {
				return
			}
			valStr := strings.TrimSpace(cells.Eq(amountCol).Text())
			valStr = strings.ReplaceAll(valStr, ",", "")
			valStr = strings.ReplaceAll(valStr, "元", "")
			valStr = strings.TrimSpace(valStr)
			if val, err := strconv.ParseFloat(valStr, 64); err == nil && val > 0 {
				total += val
			}
		})
	})
	if total > 0 {
		return fmt.Sprintf("%.2f", total)
	}
	text := doc.Text()
	re := regexp.MustCompile(`维修资金列支金额[：:]?[^0-9]*([\d,]+\.?\d*)`)
	matches := re.FindStringSubmatch(text)
	if len(matches) >= 2 {
		val := strings.ReplaceAll(matches[1], ",", "")
		if v, err := strconv.ParseFloat(val, 64); err == nil && v > 100 && v < 50000000 {
			return fmt.Sprintf("%.2f", v)
		}
	}
	re2 := regexp.MustCompile(`拨付金额[：:]?\s*([\d,]+\.?\d*)`)
	matches2 := re2.FindStringSubmatch(text)
	if len(matches2) >= 2 {
		val := strings.ReplaceAll(matches2[1], ",", "")
		if v, err := strconv.ParseFloat(val, 64); err == nil && v > 100 && v < 50000000 {
			return fmt.Sprintf("%.2f", v)
		}
	}
	re3 := regexp.MustCompile(`[￥¥][：:]?\s*([\d,]+\.?\d*)`)
	matches3 := re3.FindStringSubmatch(text)
	if len(matches3) >= 2 {
		val := strings.ReplaceAll(matches3[1], ",", "")
		if v, err := strconv.ParseFloat(val, 64); err == nil && v > 100 && v < 50000000 {
			return fmt.Sprintf("%.2f", v)
		}
	}
	start := strings.Index(text, "维修资金列支金额")
	if start >= 0 {
		section := text[start:]
		end := strings.Index(section, "公示时间")
		if end > 0 {
			section = section[:end]
		}
		reAll := regexp.MustCompile(`(\d+\.?\d*)`)
		allNums := reAll.FindAllStringSubmatch(section, -1)
		var sum float64
		for _, m := range allNums {
			v, err := strconv.ParseFloat(m[1], 64)
			if err == nil && v > 100 && v < 50000000 {
				sum += v
			}
		}
		if sum > 0 {
			return fmt.Sprintf("%.2f", sum)
		}
	}
	return ""
}

func insertArticle(db *sql.DB, article Article) {
	query := `INSERT OR REPLACE INTO articles (title, url, publish_date, content, amount) VALUES (?, ?, ?, ?, ?)`
	_, err := db.Exec(query, article.Title, article.URL, article.Date, article.Content, article.Amount)
	if err != nil {
		log.Printf("Error inserting article: %v", err)
	}
}

func createSubscriptionsTable(db *sql.DB) {
	db.Exec(`CREATE TABLE IF NOT EXISTS subscriptions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT NOT NULL,
		community TEXT NOT NULL,
		unsub_token TEXT NOT NULL DEFAULT '',
		last_article_id INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(email, community)
	)`)
	db.Exec("CREATE INDEX IF NOT EXISTS idx_unsub_token ON subscriptions(unsub_token)")
	db.Exec("ALTER TABLE subscriptions ADD COLUMN unsub_token TEXT NOT NULL DEFAULT ''")
	db.Exec("UPDATE subscriptions SET unsub_token = hex(randomblob(16)) WHERE unsub_token = ''")
}

func genToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func sendMail(to, subject, htmlBody string) error {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", "smtp.qq.com:465", &tls.Config{ServerName: "smtp.qq.com"})
	if err != nil {
		return fmt.Errorf("tls dial: %v", err)
	}
	defer conn.Close()
	client, err := smtp.NewClient(conn, "smtp.qq.com")
	if err != nil {
		return fmt.Errorf("smtp client: %v", err)
	}
	defer client.Quit()
	auth := smtp.PlainAuth("", smtpEmail, smtpPass, "smtp.qq.com")
	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("auth: %v", err)
	}
	if err = client.Mail(smtpEmail); err != nil {
		return fmt.Errorf("mail from: %v", err)
	}
	if err = client.Rcpt(to); err != nil {
		return fmt.Errorf("rcpt: %v", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("data: %v", err)
	}
	msg := "From: " + smtpEmail + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=UTF-8\r\n" +
		"\r\n" +
		htmlBody + "\r\n"
	_, err = w.Write([]byte(msg))
	w.Close()
	return err
}

func notifySubscribers(db *sql.DB) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("notifySubscribers panic: %v", r)
		}
	}()

	var maxID int
	db.QueryRow("SELECT COALESCE(MAX(id), 0) FROM articles").Scan(&maxID)
	if maxID == 0 {
		return
	}
	rows, err := db.Query("SELECT id, email, community, unsub_token, last_article_id FROM subscriptions")
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var subID, lastID int
		var email, community, token string
		rows.Scan(&subID, &email, &community, &token, &lastID)
		if maxID <= lastID {
			continue
		}
		articleRows, err := db.Query(
				"SELECT id, title, amount, url, content FROM articles WHERE id > ? AND id <= ? AND title LIKE ? ORDER BY id",
				lastID, maxID, "%"+community+"%")
		if err != nil {
			continue
		}
		var matches []struct{ id int; title, amount, url, content string }
		for articleRows.Next() {
			var m struct{ id int; title, amount, url, content string }
			articleRows.Scan(&m.id, &m.title, &m.amount, &m.url, &m.content)
			matches = append(matches, m)
		}
		articleRows.Close()
		if len(matches) == 0 {
			db.Exec("UPDATE subscriptions SET last_article_id = ? WHERE id = ?", maxID, subID)
			continue
		}
		unsubURL := fmt.Sprintf("%s/api/unsubscribe?token=%s", baseURL, token)
		var subject string
		if len(matches) == 1 {
			subject = fmt.Sprintf("【维修资金拨付】%s", matches[0].title)
		} else {
			subject = fmt.Sprintf("【维修资金拨付】%s 等 %d 条新公示", community, len(matches))
		}
		var parts []string
		for _, a := range matches {
			parts = append(parts, fmt.Sprintf(
				"<h2>%s</h2><p><strong>金额：</strong>%s 元</p><pre style=\"font-size:14px;line-height:1.8;white-space:pre-wrap;background:#f5f5f5;padding:16px;border-radius:8px\">%s</pre><p><a href=\"%s\" style=\"color:#1a73e8\">查看原文 →</a></p>",
				html.EscapeString(a.title), html.EscapeString(a.amount), html.EscapeString(a.content), a.url))
		}
		parts = append(parts, fmt.Sprintf("<hr><p style=\"color:#888;font-size:12px\"><a href=\"%s\">取消订阅</a></p>", unsubURL))
		body := strings.Join(parts, "\n<hr>\n")
		log.Printf("Sending email to %s for %d articles", email, len(matches))
		if err := sendMail(email, subject, body); err != nil {
			log.Printf("Email error to %s: %v", email, err)
		}
		db.Exec("UPDATE subscriptions SET last_article_id = ? WHERE id = ?", maxID, subID)
	}
}
