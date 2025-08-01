package repository

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"golang-stock-scryper/internal/executor/config"
	"golang-stock-scryper/internal/executor/dto"
	"golang-stock-scryper/pkg/logger"
	"golang-stock-scryper/pkg/utils"
	"io"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/time/rate"
)

type YahooFinanceRepository interface {
	Get(ctx context.Context, param dto.GetStockDataParam) (*dto.StockData, error)
	GetMultiTimeframe(ctx context.Context, stockCode string) (*dto.StockDataMultiTimeframe, error)
}

// yahooFinanceRepository is an implementation of NewsAnalyzerRepository that uses the Google Gemini API.
type yahooFinanceRepository struct {
	client         *http.Client
	cfg            *config.Config
	logger         *logger.Logger
	requestLimiter *rate.Limiter
}

// NewYahooFinanceRepository creates a new instance of yahooFinanceRepository.
func NewYahooFinanceRepository(cfg *config.Config, log *logger.Logger) (YahooFinanceRepository, error) {
	secondsPerRequest := time.Minute / time.Duration(cfg.YahooFinance.MaxRequestPerMinute)
	requestLimiter := rate.NewLimiter(rate.Every(secondsPerRequest), 1)

	return &yahooFinanceRepository{
		client:         &http.Client{},
		cfg:            cfg,
		logger:         log,
		requestLimiter: requestLimiter,
	}, nil
}

func (r *yahooFinanceRepository) GetMultiTimeframe(ctx context.Context, stockCode string) (*dto.StockDataMultiTimeframe, error) {
	stockData1d, err := r.Get(ctx, dto.GetStockDataParam{
		StockCode: stockCode,
		Range:     "3m",
		Interval:  "1d",
	})
	if err != nil {
		return nil, err
	}
	stockData4h, err := r.Get(ctx, dto.GetStockDataParam{
		StockCode: stockCode,
		Range:     "1m",
		Interval:  "4h",
	})
	if err != nil {
		return nil, err
	}
	stockData1h, err := r.Get(ctx, dto.GetStockDataParam{
		StockCode: stockCode,
		Range:     "14d",
		Interval:  "1h",
	})
	if err != nil {
		return nil, err
	}
	return &dto.StockDataMultiTimeframe{
		MarketPrice: stockData1d.MarketPrice,
		OHLCV1D:     stockData1d.OHLCV,
		OHLCV4H:     stockData4h.OHLCV,
		OHLCV1H:     stockData1h.OHLCV,
	}, nil
}

func (r *yahooFinanceRepository) Get(ctx context.Context, param dto.GetStockDataParam) (*dto.StockData, error) {
	if err := r.requestLimiter.Wait(ctx); err != nil {
		return nil, err
	}
	// Add .JK suffix for Indonesian stocks
	param.StockCode = param.StockCode + ".JK"

	// Build URL with query parameters
	baseURL := r.cfg.YahooFinance.BaseURL + "/" + param.StockCode
	params := url.Values{}

	period1, period2 := r.MapPeriodeStringToUnix(param.Range)
	if period1 == 0 || period2 == 0 {
		return nil, fmt.Errorf("invalid period")
	}
	params.Add("period1", fmt.Sprintf("%d", period1))
	params.Add("period2", fmt.Sprintf("%d", period2))
	params.Add("interval", param.Interval)
	params.Add("includePrePost", "false")
	params.Add("events", "div,split")

	requestURL := baseURL + "?" + params.Encode()

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers to mimic browser request
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Referer", "https://finance.yahoo.com/")

	r.logger.Debug("Requesting data from Yahoo Finance", logger.StringField("url", requestURL))
	// Make HTTP request
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data from yahoo finance: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("yahoo finance api returned status: %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Handle gzip compression
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(io.NopCloser(io.NewSectionReader(bytes.NewReader(body), 0, int64(len(body)))))
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer reader.Close()

		body, err = io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress gzip response: %w", err)
		}
	}

	// Parse JSON response
	var yahooResp dto.YahooFinanceResponse
	if err := json.Unmarshal(body, &yahooResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Yahoo Finance response: %w", err)
	}

	// Check for API errors
	if yahooResp.Chart.Error != nil {
		return nil, fmt.Errorf("yahoo finance api error: %v", yahooResp.Chart.Error)
	}

	// Check if we have results
	if len(yahooResp.Chart.Result) == 0 {
		return nil, fmt.Errorf("no data returned for symbol: %s", param.StockCode)
	}

	result := yahooResp.Chart.Result[0]
	if len(result.Indicators.Quote) == 0 {
		return nil, fmt.Errorf("no quote data available for symbol: %s", param.StockCode)
	}

	quote := result.Indicators.Quote[0]

	// Convert to OHLCVData format
	var ohlcvData []dto.StockOHLCV
	for i, timestamp := range result.Timestamp {
		// Skip if any required data is missing
		if i >= len(quote.Open) || i >= len(quote.High) || i >= len(quote.Low) ||
			i >= len(quote.Close) || i >= len(quote.Volume) {
			continue
		}

		// Skip if any value is 0 (missing data)
		if quote.Open[i] == 0 || quote.High[i] == 0 || quote.Low[i] == 0 ||
			quote.Close[i] == 0 {
			continue
		}

		ohlcvData = append(ohlcvData, dto.StockOHLCV{
			Timestamp: timestamp,
			Open:      quote.Open[i],
			High:      quote.High[i],
			Low:       quote.Low[i],
			Close:     quote.Close[i],
			Volume:    quote.Volume[i],
		})
	}

	if len(ohlcvData) == 0 {
		return nil, fmt.Errorf("no valid OHLCV data found for symbol: %s", param.StockCode)
	}

	marketPrice := 0.0

	if len(yahooResp.Chart.Result) > 0 && yahooResp.Chart.Result[0].Meta.RegularMarketPrice > 0 {
		marketPrice = yahooResp.Chart.Result[0].Meta.RegularMarketPrice
	}

	return &dto.StockData{
		MarketPrice: marketPrice,
		OHLCV:       ohlcvData,
	}, nil
}

// MapPeriodeStringToUnix convert days to unix timestamp
func (r *yahooFinanceRepository) MapPeriodeStringToUnix(periode string) (int64, int64) {

	now := utils.TimeNowWIB()
	switch periode {
	case "1d":
		return now.AddDate(0, 0, -1).Unix(), now.Unix()
	case "14d":
		return now.AddDate(0, 0, -14).Unix(), now.Unix()
	case "1w":
		return now.AddDate(0, 0, -7).Unix(), now.Unix()
	case "1m":
		return now.AddDate(0, 0, -30).Unix(), now.Unix()
	case "2m":
		return now.AddDate(0, 0, -60).Unix(), now.Unix()
	case "3m":
		return now.AddDate(0, 0, -90).Unix(), now.Unix()
	case "6m":
		return now.AddDate(0, 0, -180).Unix(), now.Unix()
	case "1y":
		return now.AddDate(0, 0, -365).Unix(), now.Unix()
	default:
		return 0, 0
	}
}
