package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"golang-stock-scryper/internal/executor/config"
	"golang-stock-scryper/internal/executor/dto"
	"golang-stock-scryper/pkg/logger"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type TradingViewRepository interface {
	GetStockBuyList(ctx context.Context, payload map[string]interface{}) ([]string, error)
}

type tradingViewRepository struct {
	cfg            *config.Config
	log            *logger.Logger
	httpClient     *http.Client
	requestLimiter *rate.Limiter
}

func NewTradingViewRepository(cfg *config.Config, log *logger.Logger) TradingViewRepository {
	secondsPerRequest := time.Minute / time.Duration(cfg.TradingView.MaxRequestPerMinute)
	requestLimiter := rate.NewLimiter(rate.Every(secondsPerRequest), 1)
	return &tradingViewRepository{
		cfg: cfg,
		log: log,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		requestLimiter: requestLimiter,
	}
}

func (r *tradingViewRepository) GetStockBuyList(ctx context.Context, payload map[string]interface{}) ([]string, error) {
	url := r.cfg.TradingView.BaseURL + "/indonesia/scan?label-product=screener-stock"
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	body, err := r.sendRequest(ctx, "POST", url, string(jsonPayload))
	if err != nil {
		return nil, err
	}

	var response dto.TradingViewResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	var stockCodes []string
	for _, v := range response.Data {
		if len(stockCodes) >= r.cfg.TradingView.BuyListMaxStockAnalyze {
			break
		}
		if v.StockCode == "" {
			continue
		}

		valueParse := strings.Split(v.StockCode, ":")
		if len(valueParse) < 2 {
			continue
		}

		if len(v.TechnicalRating) == 0 {
			continue
		}

		stockCode := valueParse[1]
		if v.TechnicalRating[0] >= r.cfg.TradingView.BuyListMinTechnicalRating {
			stockCodes = append(stockCodes, stockCode)
		}
	}

	r.log.DebugContext(ctx, "TradingView Found stock codes", logger.StringField("stock_codes", strings.Join(stockCodes, ", ")))

	return stockCodes, nil
}

func (r *tradingViewRepository) sendRequest(ctx context.Context, method string, url string, jsonStr string) ([]byte, error) {
	fields := []zap.Field{
		zap.String("url", url),
		zap.Int("max_request_per_minute", r.cfg.TradingView.MaxRequestPerMinute),
		zap.Int("delay", int(r.requestLimiter.Reserve().Delay())),
		zap.String("payload", jsonStr),
	}

	if err := r.requestLimiter.Wait(ctx); err != nil {
		fields = append(fields, zap.Error(err))
		r.log.ErrorContext(ctx, "Failed to wait for request limit", fields...)
		return nil, err
	}

	var payload *bytes.Buffer
	if jsonStr != "" {
		payload = bytes.NewBufferString(jsonStr)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, payload)
	if err != nil {
		fields = append(fields, zap.Error(err))
		r.log.ErrorContext(ctx, "Failed to create new http request", fields...)
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		fields = append(fields, zap.Error(err))
		r.log.ErrorContext(ctx, "Failed to send request to TradingView API", fields...)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fields = append(fields, zap.Int("status_code", resp.StatusCode))
		r.log.ErrorContext(ctx, "Received non-OK response from TradingView API", fields...)
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fields = append(fields, zap.Error(err))
		r.log.ErrorContext(ctx, "Failed to read response body from TradingView API", fields...)
		return nil, err
	}

	return body, nil
}
