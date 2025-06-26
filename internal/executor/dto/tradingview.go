package dto

type TradingViewResponse struct {
	TotalCount int                       `json:"totalCount"`
	Data       []TradingViewDataResponse `json:"data"`
}

type TradingViewDataResponse struct {
	// assume column only :
	//   "columns": [
	//       "Recommend.All"
	//   ],
	StockCode       string    `json:"s"`
	TechnicalRating []float64 `json:"d"`
}
