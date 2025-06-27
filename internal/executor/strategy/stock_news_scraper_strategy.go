package strategy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"golang-stock-scryper/internal/entity"
	"golang-stock-scryper/internal/executor/dto"
	"golang-stock-scryper/internal/executor/repository"
	"golang-stock-scryper/pkg/decoder"
	"golang-stock-scryper/pkg/logger"
	"golang-stock-scryper/pkg/utils"
	"net/http"
	"net/url"

	"github.com/PuerkitoBio/goquery"
	"github.com/mauidude/go-readability"
	"github.com/patrickmn/go-cache"
	"gorm.io/gorm"
)

// StockNewsScraperStrategy defines the strategy for scraping stock news.
type StockNewsScraperStrategy struct {
	db               *gorm.DB
	logger           *logger.Logger
	decoder          *decoder.GoogleDecoder
	aiRepo           repository.AIRepository
	stockMentionRepo repository.StockMentionRepository
	stockNewsRepo    repository.StockNewsRepository
	client           *http.Client
	inmemoryCache    *cache.Cache
	stockRepo        repository.StocksRepository
}

// NewStockNewsScraperStrategy creates a new instance of StockNewsScraperStrategy.
func NewStockNewsScraperStrategy(db *gorm.DB, logger *logger.Logger, decoder *decoder.GoogleDecoder, aiRepo repository.AIRepository, stockMentionRepo repository.StockMentionRepository, stockNewsRepo repository.StockNewsRepository, stockRepo repository.StocksRepository) *StockNewsScraperStrategy {
	return &StockNewsScraperStrategy{
		db:               db,
		logger:           logger,
		decoder:          decoder,
		aiRepo:           aiRepo,
		stockMentionRepo: stockMentionRepo,
		stockNewsRepo:    stockNewsRepo,
		client:           &http.Client{},
		inmemoryCache:    cache.New(5*time.Minute, 10*time.Minute),
		stockRepo:        stockRepo,
	}
}

// GetType returns the job type this strategy handles.
func (s *StockNewsScraperStrategy) GetType() entity.JobType {
	return entity.JobTypeStockNewsScraper
}

// Execute runs the stock news scraping job.
type scrapeResult struct {
	Status      string   `json:"status"`
	FailedLinks []string `json:"failed_links"`
	Errors      []string `json:"errors"`
	QueryRSS    string   `json:"query_rss"`
}

type StockNewsScraperPayload struct {
	AdditionalStockCodes []string       `json:"additional_stock_codes"`
	DelayInterval        int            `json:"delay_interval"`
	MaxNews              int            `json:"max_news"`
	MaxNewsAgeInDays     int            `json:"max_news_age_in_days"`
	BlackListedDomains   []string       `json:"blacklisted_domains"`
	MaxConcurrent        int            `json:"max_concurrent"`
	AdditionalKeywords   []string       `json:"additional_keywords"`
	UseStockList         bool           `json:"use_stock_list"`
	DefaultQueryParam    string         `json:"default_query_param"`
	SourcePriority       map[string]int `json:"source_priority"`
}

func (s *StockNewsScraperStrategy) Execute(ctx context.Context, job *entity.Job) (string, error) {
	var payload StockNewsScraperPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return "", fmt.Errorf("failed to unmarshal job payload: %w", err)
	}

	var results []scrapeResult
	var wg sync.WaitGroup
	var mu sync.Mutex

	defaultQueryParam := "hl=id&gl=ID&ceid=ID:id"
	if payload.DefaultQueryParam != "" {
		defaultQueryParam = payload.DefaultQueryParam
	}

	queriesRSS := []string{}

	if len(payload.AdditionalKeywords) > 0 {
		for _, keyword := range payload.AdditionalKeywords {
			if keyword == "" {
				continue
			}
			//example /search?q=invest
			queriesRSS = append(queriesRSS, fmt.Sprintf("%s&%s", keyword, defaultQueryParam))
		}
	}

	if payload.UseStockList {
		stocks, err := s.stockRepo.GetStocks(ctx)
		if err != nil {
			s.logger.Error("Failed to get stocks", logger.ErrorField(err))
			return "", fmt.Errorf("failed to get stocks: %w", err)
		}
		for _, stock := range stocks {
			queriesRSS = append(queriesRSS, fmt.Sprintf("/search?q=saham+%s&%s", stock.Code, defaultQueryParam))
		}
	}

	if len(payload.AdditionalStockCodes) > 0 {
		for _, stockCode := range payload.AdditionalStockCodes {
			queriesRSS = append(queriesRSS, fmt.Sprintf("/search?q=saham+%s&%s", stockCode, defaultQueryParam))
		}
	}

	semaphore := make(chan struct{}, payload.MaxConcurrent)

	for _, queryRSS := range queriesRSS {
		if !utils.ShouldContinue(ctx, s.logger) {
			break
		}
		wg.Add(1)
		utils.GoSafe(func() {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			scrapeResultData := scrapeResult{
				FailedLinks: []string{},
				QueryRSS:    queryRSS,
				Errors:      []string{},
			}
			url := fmt.Sprintf("https://news.google.com/rss%s", queryRSS)
			rss, err := s.parseRSSFeed(ctx, url)
			if err != nil {
				s.logger.Error("Failed to parse RSS feed", logger.ErrorField(err), logger.StringField("query_rss", queryRSS))
				scrapeResultData.Status = FAILED
				scrapeResultData.Errors = append(scrapeResultData.Errors, err.Error())
				mu.Lock()
				results = append(results, scrapeResultData)
				mu.Unlock()
				return
			}
			s.logger.Info("Processing RSS feed", logger.StringField("url", url))

			// Filter out existing news items
			filteredItems, err := s.filterExistingNewsItems(ctx, rss.Channel.Items, payload.MaxNewsAgeInDays)
			if err != nil {
				s.logger.Error("Failed to filter existing news items", logger.ErrorField(err), logger.StringField("query_rss", queryRSS))
				scrapeResultData.Status = FAILED
				scrapeResultData.Errors = append(scrapeResultData.Errors, err.Error())
				mu.Lock()
				results = append(results, scrapeResultData)
				mu.Unlock()
			}

			// Sort items by published date descending
			s.sortItems(filteredItems, payload.SourcePriority)

			s.logger.Info("Filtered news items",
				logger.IntField("original_count", len(rss.Channel.Items)),
				logger.IntField("filtered_count", len(filteredItems)),
				logger.StringField("query_rss", queryRSS),
			)

			countSuccess := 0
			for _, item := range filteredItems {

				if !utils.ShouldContinue(ctx, s.logger) {
					return
				}

				s.logger.Info("Processing news item",
					logger.StringField("title", item.Title),
					logger.StringField("query_rss", queryRSS),
					logger.IntField("count_success", countSuccess),
					logger.IntField("count_total", len(rss.Channel.Items)),
					logger.IntField("max_news", payload.MaxNews),
				)
				if countSuccess >= payload.MaxNews {
					break
				}

				status, news, err := s.processNewsItem(ctx, &item, queryRSS, payload)
				if err != nil {
					scrapeResultData.FailedLinks = append(scrapeResultData.FailedLinks, news.Link)
					scrapeResultData.Errors = append(scrapeResultData.Errors, err.Error())
					s.logger.Error("Failed to process news item", logger.ErrorField(err), logger.StringField("title", item.Title))
					continue
				}

				if status == FAILED {
					scrapeResultData.FailedLinks = append(scrapeResultData.FailedLinks, news.Link)
					continue
				}
				countSuccess++
				time.Sleep(time.Duration(payload.DelayInterval) * time.Second)
			}

			if len(scrapeResultData.FailedLinks) == 0 {
				scrapeResultData.Status = SUCCESS
			} else if countSuccess == 0 {
				scrapeResultData.Status = SKIPPED
			} else {
				scrapeResultData.Status = FAILED
			}
			mu.Lock()
			results = append(results, scrapeResultData)
			mu.Unlock()
		})

	}

	wg.Wait()

	resultJSON, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("failed to marshal results: %w", err)
	}

	return string(resultJSON), nil
}

// filterExistingNewsItems filters out feed items that already exist in the database based on their hash identifiers
func (s *StockNewsScraperStrategy) filterExistingNewsItems(ctx context.Context, items []dto.RSSItem, maxNewsAgeInDays int) ([]dto.RSSItem, error) {
	if len(items) == 0 {
		return items, nil
	}

	// Create a map to store the hash identifiers of all items
	hashMap := make(map[string]dto.RSSItem)
	var hashStrings []string

	// Generate hash for each item and store in map
	for _, item := range items {
		hashIdentifier := sha256.Sum256([]byte(item.Link + "|" + item.PubDate.String()))
		hashString := hex.EncodeToString(hashIdentifier[:])
		hashMap[hashString] = item
		hashStrings = append(hashStrings, hashString)
	}

	// fetch the existing news
	var existingNews []entity.StockNews
	err := s.db.WithContext(ctx).Table("stock_news").Select("id", "hash_identifier").
		Where("hash_identifier IN ?", hashStrings).
		Find(&existingNews).Error

	if err != nil {
		s.logger.Error("Failed to fetch existing news", logger.ErrorField(err))
		return nil, fmt.Errorf("failed to fetch existing news: %w", err)
	}

	existingHashes := make(map[string]bool)
	for _, news := range existingNews {
		existingHashes[news.HashIdentifier] = true
	}

	now := utils.TimeNowWIB()

	// Filter out existing items
	var filteredItems []dto.RSSItem
	for hash, item := range hashMap {
		if existingHashes[hash] {
			s.logger.Info("News already exists", logger.StringField("rss", item.Link), logger.StringField("hash", hash))
			continue
		}

		if item.PubDate == nil {
			s.logger.Info("News published date is nil", logger.StringField("rss", item.Link))
			continue
		}
		if item.PubDate.Time().In(utils.GetWibTimeLocation()).Before(now.Add(-time.Duration(maxNewsAgeInDays*24) * time.Hour)) {
			s.logger.Debug("News is too old",
				logger.StringField("title", item.Title),
				logger.StringField("published_date", item.PubDate.Time().Format("2006-01-02 15:04:05")),
				logger.IntField("max_news_age_in_days", maxNewsAgeInDays))
			continue
		}

		filteredItems = append(filteredItems, item)
	}

	return filteredItems, nil
}

func (s *StockNewsScraperStrategy) processNewsItem(ctx context.Context, item *dto.RSSItem, queryRSS string, payload StockNewsScraperPayload) (string, entity.StockNews, error) {
	decodeResult := s.decoder.DecodeGoogleNewsURL(item.Link, 0)
	if !decodeResult.Status {
		s.logger.Error("Failed to decode google rss link", logger.StringField("message", decodeResult.Message))
		return FAILED, entity.StockNews{}, fmt.Errorf("failed to decode google rss link: %s", decodeResult.Message)
	}
	decodedURL := decodeResult.DecodedURL

	publishedDateStr := "N/A"
	if item.PubDate == nil {
		s.logger.Error("Failed to parse published date", logger.StringField("link", decodedURL))
		return FAILED, entity.StockNews{}, fmt.Errorf("failed to parse published date")
	}

	publishedDateStr = item.PubDate.Time().Format(time.RFC3339)

	hashIdentifier := sha256.Sum256([]byte(item.Link + "|" + publishedDateStr))
	hashString := hex.EncodeToString(hashIdentifier[:])

	news := entity.StockNews{
		Title:          utils.CleanToValidUTF8(item.Title),
		Link:           decodedURL,
		PublishedAt:    utils.ToPointer(item.PubDate.Time()),
		HashIdentifier: hashString,
		GoogleRSSLink:  item.Link,
		KeywordRSS:     queryRSS,
		SourceName:     item.Source.Name,
	}

	parsedURL, err := url.Parse(decodedURL)
	if err != nil {
		s.logger.Error("Could not parse decoded URL to get hostname", logger.StringField("url", decodedURL), logger.ErrorField(err))
		return FAILED, entity.StockNews{}, fmt.Errorf("failed to parse decoded URL: %w", err)
	}
	news.Source = parsedURL.Hostname()

	if utils.ContainsString(payload.BlackListedDomains, parsedURL.Hostname()) {
		s.logger.Warn("Skip news from blacklisted domain", logger.StringField("domain", parsedURL.Hostname()), logger.StringField("query_rss", queryRSS))
		return SKIPPED, news, nil
	}

	rawContent, err := s.generateContent(ctx, decodedURL)
	if err != nil {
		s.logger.Error("Failed to generate raw content", logger.ErrorField(err), logger.StringField("url", decodedURL))
		return FAILED, entity.StockNews{}, fmt.Errorf("failed to generate raw content: %w", err)
	}
	news.RawContent = rawContent

	var analysisResult *dto.NewsAnalysisResult

	analysisResult, err = s.aiRepo.NewsAnalyze(ctx, news.Title, publishedDateStr, news.RawContent)
	if err != nil {
		s.logger.Error("Failed to analyze news content", logger.ErrorField(err), logger.StringField("title", item.Title))
		return FAILED, entity.StockNews{}, fmt.Errorf("failed to analyze news content: %w", err)
	}

	if analysisResult == nil {
		s.logger.Error("Failed to analyze news content return nil", logger.StringField("link", news.Link))
		return FAILED, entity.StockNews{}, fmt.Errorf("failed to analyze news content")
	}

	news.ImpactScore = analysisResult.ImpactScore
	news.Summary = analysisResult.Summary
	news.KeyIssue = analysisResult.KeyIssue
	for _, stockMention := range analysisResult.StockMentions {
		news.StockMentions = append(news.StockMentions, entity.StockMention{
			StockCode:       stockMention.StockCode,
			Sentiment:       stockMention.Sentiment,
			ConfidenceScore: stockMention.ConfidenceScore,
			Impact:          stockMention.Impact,
			Reason:          stockMention.Reason,
		})
	}

	err = s.stockNewsRepo.CreateIgnoreConflict(ctx, &news)

	if err != nil {
		s.logger.Error("Failed to create stock news", logger.ErrorField(err), logger.StringField("link", news.Link))
		return FAILED, news, fmt.Errorf("failed to create stock news: %w", err)
	}

	return SUCCESS, news, nil
}

func (s *StockNewsScraperStrategy) generateContent(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		s.logger.Error("Failed to create request", logger.ErrorField(err), logger.StringField("url", url))
		return "", fmt.Errorf("failed to create request for news item: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	resp, err := s.client.Do(req)
	if err != nil {
		s.logger.Error("Failed to fetch news content", logger.ErrorField(err), logger.StringField("url", url))
		return "", fmt.Errorf("failed to fetch news content: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.logger.Error("Failed to fetch news content with non-200 status", logger.IntField("status", resp.StatusCode), logger.StringField("url", url))
		return "", fmt.Errorf("failed to fetch news content, status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logger.Error("Failed to read response body", logger.ErrorField(err), logger.StringField("url", url))
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	doc, err := readability.NewDocument(string(body))
	if err != nil {
		s.logger.Error("Failed to parse news content", logger.ErrorField(err), logger.StringField("url", url))
		return "", fmt.Errorf("failed to parse news content: %w", err)
	}
	content := doc.Content()
	docHTML, err := goquery.NewDocumentFromReader(bytes.NewReader([]byte(content)))
	if err != nil {
		s.logger.Error("Failed to parse news content", logger.ErrorField(err), logger.StringField("url", url))
		return "", fmt.Errorf("failed to parse news content: %w", err)
	}

	content = strings.TrimSpace(docHTML.Text())
	content = strings.ReplaceAll(content, "\n", "")
	content = strings.ReplaceAll(content, "\t", "")
	content = strings.ReplaceAll(content, "\r", "")
	content = strings.ReplaceAll(content, "\f", "")
	return utils.SafeText(content), nil
}

func (s *StockNewsScraperStrategy) parseRSSFeed(ctx context.Context, url string) (*dto.RSS, error) {
	var rss dto.RSS

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		s.logger.Error("Failed to create request", logger.ErrorField(err), logger.StringField("url", url))
		return nil, fmt.Errorf("failed to create request for RSS feed: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	resp, err := s.client.Do(req)
	if err != nil {
		s.logger.Error("Failed to fetch parse RSS feed", logger.ErrorField(err), logger.StringField("url", url))
		return nil, fmt.Errorf("failed to fetch parse RSS feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.logger.Error("Failed to fetch parse RSS feed with non-200 status", logger.IntField("status", resp.StatusCode), logger.StringField("url", url))
		return nil, fmt.Errorf("failed to fetch parse RSS feed, status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logger.Error("Failed to read response body", logger.ErrorField(err), logger.StringField("url", url))
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	err = xml.Unmarshal(body, &rss)
	if err != nil {
		s.logger.Error("Failed to unmarshal RSS feed", logger.ErrorField(err), logger.StringField("url", url))
		return nil, fmt.Errorf("failed to unmarshal RSS feed: %w", err)
	}

	return &rss, nil
}

func (s *StockNewsScraperStrategy) sortItems(items []dto.RSSItem, sourcePriority map[string]int) {
	sort.SliceStable(items, func(i, j int) bool {

		si := "Unknown"
		if items[i].Source != nil {
			si = items[i].Source.URL
		}
		sj := "Unknown"
		if items[j].Source != nil {
			sj = items[j].Source.URL
		}

		pi, ok := sourcePriority[si]
		if !ok {
			pi = 999
		}
		pj, ok := sourcePriority[sj]
		if !ok {
			pj = 999
		}

		// Step 1: Prioritas source lebih tinggi
		if pi != pj {
			return pi < pj
		}

		// Step 2: Kalau prioritas sama, bandingkan tanggal (terbaru dulu)
		ti := items[i].PubDate.Time()
		tj := items[j].PubDate.Time()

		return ti.After(tj)
	})
}
