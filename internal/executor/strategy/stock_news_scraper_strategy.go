package strategy

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
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
	"github.com/mmcdole/gofeed"
	"github.com/patrickmn/go-cache"
	"gorm.io/gorm"
)

const (
	SUCCESS = "success"
	FAILED  = "failed"
	SKIPPED = "skipped"
)

// StockNewsScraperStrategy defines the strategy for scraping stock news.
type StockNewsScraperStrategy struct {
	db               *gorm.DB
	logger           *logger.Logger
	decoder          *decoder.GoogleDecoder
	analyzerRepo     repository.NewsAnalyzerRepository
	stockMentionRepo repository.StockMentionRepository
	stockNewsRepo    repository.StockNewsRepository
	client           *http.Client
	inmemoryCache    *cache.Cache
}

// NewStockNewsScraperStrategy creates a new instance of StockNewsScraperStrategy.
func NewStockNewsScraperStrategy(db *gorm.DB, logger *logger.Logger, decoder *decoder.GoogleDecoder, analyzerRepo repository.NewsAnalyzerRepository, stockMentionRepo repository.StockMentionRepository, stockNewsRepo repository.StockNewsRepository) *StockNewsScraperStrategy {
	return &StockNewsScraperStrategy{
		db:               db,
		logger:           logger,
		decoder:          decoder,
		analyzerRepo:     analyzerRepo,
		stockMentionRepo: stockMentionRepo,
		stockNewsRepo:    stockNewsRepo,
		client:           &http.Client{},
		inmemoryCache:    cache.New(5*time.Minute, 10*time.Minute),
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
	StockCode   string   `json:"stock_code"`
}

type StockNewsScraperPayload struct {
	StockCodes         []string `json:"stock_codes"`
	DelayInterval      int      `json:"delay_interval"`
	MaxNews            int      `json:"max_news"`
	MaxNewsAgeInDays   int      `json:"max_news_age_in_days"`
	BlackListedDomains []string `json:"blacklisted_domains"`
}

func (s *StockNewsScraperStrategy) Execute(ctx context.Context, job *entity.Job) (string, error) {
	var payload StockNewsScraperPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return "", fmt.Errorf("failed to unmarshal job payload: %w", err)
	}

	var results []scrapeResult
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, stockCode := range payload.StockCodes {
		wg.Add(1)
		code := stockCode
		utils.GoSafe(func() {
			defer wg.Done()
			scrapeResultData := scrapeResult{
				FailedLinks: []string{},
				StockCode:   code,
				Errors:      []string{},
			}
			url := fmt.Sprintf("https://news.google.com/rss/search?q=saham+%s&hl=id&gl=ID&ceid=ID:id", code)
			fp := gofeed.NewParser()
			feed, err := fp.ParseURLWithContext(url, ctx)
			if err != nil {
				s.logger.Error("Failed to parse RSS feed", logger.ErrorField(err), logger.StringField("stock_code", code))
				scrapeResultData.Status = FAILED
				scrapeResultData.Errors = append(scrapeResultData.Errors, err.Error())
				mu.Lock()
				results = append(results, scrapeResultData)
				mu.Unlock()
				return
			}
			// Sort items by published date descending
			sort.Slice(feed.Items, func(i, j int) bool {
				if feed.Items[i].PublishedParsed == nil || feed.Items[j].PublishedParsed == nil {
					return false
				}
				return feed.Items[i].PublishedParsed.After(*feed.Items[j].PublishedParsed)
			})

			// Filter out existing news items
			filteredItems, err := s.filterExistingNewsItems(ctx, feed.Items, payload.MaxNewsAgeInDays)
			if err != nil {
				s.logger.Error("Failed to filter existing news items", logger.ErrorField(err), logger.StringField("stock_code", code))
				scrapeResultData.Status = FAILED
				scrapeResultData.Errors = append(scrapeResultData.Errors, err.Error())
				mu.Lock()
				results = append(results, scrapeResultData)
				mu.Unlock()
			}

			s.logger.Info("Filtered news items",
				logger.IntField("original_count", len(feed.Items)),
				logger.IntField("filtered_count", len(filteredItems)),
				logger.StringField("stock_code", code),
			)

			countSuccess := 0
			for _, item := range filteredItems {
				select {
				case <-ctx.Done():
					s.logger.Info("Context cancelled, stopping news processing for stock", logger.StringField("stock_code", code))
					return
				default:
				}

				s.logger.Info("Processing news item",
					logger.StringField("title", item.Title),
					logger.StringField("stock_code", code),
					logger.IntField("count_success", countSuccess),
					logger.IntField("count_total", len(feed.Items)),
					logger.IntField("max_news", payload.MaxNews),
				)
				if countSuccess >= payload.MaxNews {
					break
				}

				status, news, err := s.processNewsItem(ctx, item, code, payload)
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
func (s *StockNewsScraperStrategy) filterExistingNewsItems(ctx context.Context, items []*gofeed.Item, maxNewsAgeInDays int) ([]*gofeed.Item, error) {
	if len(items) == 0 {
		return items, nil
	}

	// Create a map to store the hash identifiers of all items
	hashMap := make(map[string]*gofeed.Item)
	var hashStrings []string

	// Generate hash for each item and store in map
	for _, item := range items {
		hashIdentifier := md5.Sum([]byte(item.Link + "|" + item.Published))
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
	var filteredItems []*gofeed.Item
	for hash, item := range hashMap {
		if existingHashes[hash] {
			continue
		}

		if item.PublishedParsed == nil {
			continue
		}

		if item.PublishedParsed.Before(now.Add(-time.Duration(maxNewsAgeInDays*24) * time.Hour)) {
			continue
		}

		filteredItems = append(filteredItems, item)
	}

	return filteredItems, nil
}

func (s *StockNewsScraperStrategy) processNewsItem(ctx context.Context, item *gofeed.Item, stockCode string, payload StockNewsScraperPayload) (string, entity.StockNews, error) {
	decodeResult := s.decoder.DecodeGoogleNewsURL(item.Link, 0)
	if !decodeResult.Status {
		s.logger.Error("Failed to decode google rss link", logger.StringField("message", decodeResult.Message))
		return FAILED, entity.StockNews{}, fmt.Errorf("failed to decode google rss link: %s", decodeResult.Message)
	}
	decodedURL := decodeResult.DecodedURL

	publishedDateStr := "N/A"
	if item.PublishedParsed == nil {
		s.logger.Error("Failed to parse published date", logger.StringField("link", decodedURL))
		return FAILED, entity.StockNews{}, fmt.Errorf("failed to parse published date")
	}

	publishedDateStr = item.PublishedParsed.Format(time.RFC3339)

	hashIdentifier := md5.Sum([]byte(item.Link + "|" + item.Published))
	hashString := hex.EncodeToString(hashIdentifier[:])

	news := entity.StockNews{
		Title:          utils.CleanToValidUTF8(item.Title),
		Link:           decodedURL,
		PublishedAt:    item.PublishedParsed,
		HashIdentifier: hashString,
		GoogleRSSLink:  item.Link,
	}

	parsedURL, err := url.Parse(decodedURL)
	if err != nil {
		s.logger.Error("Could not parse decoded URL to get hostname", logger.StringField("url", decodedURL), logger.ErrorField(err))
		return FAILED, entity.StockNews{}, fmt.Errorf("failed to parse decoded URL: %w", err)
	}
	news.Source = parsedURL.Hostname()

	if utils.ContainsString(payload.BlackListedDomains, parsedURL.Hostname()) {
		s.logger.Warn("Skip news from blacklisted domain", logger.StringField("domain", parsedURL.Hostname()), logger.StringField("stock_code", stockCode))
		return SKIPPED, news, nil
	}

	rawContent, err := s.generateContent(ctx, decodedURL)
	if err != nil {
		s.logger.Error("Failed to generate raw content", logger.ErrorField(err), logger.StringField("url", decodedURL))
		return FAILED, entity.StockNews{}, fmt.Errorf("failed to generate raw content: %w", err)
	}
	news.RawContent = rawContent

	var analysisResult *dto.NewsAnalysisResult

	analysisResult, err = s.analyzerRepo.Analyze(ctx, stockCode, news.Title, publishedDateStr, news.RawContent)
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
