package main

import (
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	_ "github.com/mattn/go-sqlite3"
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
	flag.Parse()

	dbPath := getEnv("DB_PATH", "./bengbu_wxjj.db")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	createTable(db)

	if *serverMode {
		startServer(db)
		return
	}

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

	totalPages := 371
	log.Printf("Starting crawler: total %d pages", totalPages)

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
	for strings.Contains(s, "\n\n\n") || strings.Contains(s, "\n \n") || strings.Contains(s, "\n\t\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
		s = strings.ReplaceAll(s, "\n \n", "\n\n")
		s = strings.ReplaceAll(s, "\n\t\n", "\n\n")
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	s = strings.Join(lines, "\n")
	return strings.TrimSpace(s)
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
			if val, err := strconv.ParseFloat(valStr, 64); err == nil && val > 0 {
				total += val
			}
		})
	})
	if total > 0 {
		return fmt.Sprintf("%.2f", total)
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
