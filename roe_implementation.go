package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ROECalculator 用於計算和獲取ROE數據
type ROECalculator struct {
	client *http.Client
}

// FinancialStatement 財務報表結構
type FinancialStatement struct {
	Date       string  `json:"date"`
	StockID    string  `json:"stock_id"`
	Type       string  `json:"type"`
	Value      float64 `json:"value"`
	OriginName string  `json:"origin_name"`
}

// FinMindResponse FinMind API響應結構
type FinMindResponse struct {
	Data []FinancialStatement `json:"data"`
	Msg  string               `json:"msg"`
}

// NewROECalculator 創建ROE計算器
func NewROECalculator() *ROECalculator {
	return &ROECalculator{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CalculateROE 計算股票的ROE
func (r *ROECalculator) CalculateROE(stockCode string) (float64, error) {
	// 獲取財務報表數據 (淨利)
	netIncome, err := r.getNetIncome(stockCode)
	if err != nil {
		return 0, fmt.Errorf("獲取淨利失敗: %v", err)
	}

	// 獲取資產負債表數據 (股東權益)
	shareholderEquity, err := r.getShareholderEquity(stockCode)
	if err != nil {
		return 0, fmt.Errorf("獲取股東權益失敗: %v", err)
	}

	// 計算ROE = 淨利 / 股東權益 * 100
	if shareholderEquity == 0 {
		return 0, fmt.Errorf("股東權益為零")
	}

	roe := (netIncome / shareholderEquity) * 100
	
	fmt.Printf("股票 %s ROE計算: 淨利=%.0f, 股東權益=%.0f, ROE=%.2f%%\n", 
		stockCode, netIncome, shareholderEquity, roe)
	
	return roe, nil
}

// getNetIncome 獲取最新的淨利數據
func (r *ROECalculator) getNetIncome(stockCode string) (float64, error) {
	startDate := time.Now().AddDate(-1, 0, 0).Format("2006-01-02")
	url := fmt.Sprintf("https://api.finmindtrade.com/api/v4/data?dataset=TaiwanStockFinancialStatements&data_id=%s&start_date=%s",
		stockCode, startDate)

	resp, err := r.client.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var response FinMindResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return 0, err
	}

	// 尋找最新的淨利數據
	latestNetIncome := 0.0
	latestDate := ""

	for _, item := range response.Data {
		// 尋找淨利相關欄位
		if item.Type == "淨利（淨損）" || item.Type == "本期淨利" || 
		   item.OriginName == "淨利（淨損）" || item.OriginName == "本期淨利" {
			if item.Date > latestDate {
				latestDate = item.Date
				latestNetIncome = item.Value
			}
		}
	}

	if latestNetIncome == 0 {
		return 0, fmt.Errorf("未找到淨利數據")
	}

	return latestNetIncome, nil
}

// getShareholderEquity 獲取最新的股東權益數據
func (r *ROECalculator) getShareholderEquity(stockCode string) (float64, error) {
	startDate := time.Now().AddDate(-1, 0, 0).Format("2006-01-02")
	url := fmt.Sprintf("https://api.finmindtrade.com/api/v4/data?dataset=TaiwanStockBalanceSheet&data_id=%s&start_date=%s",
		stockCode, startDate)

	resp, err := r.client.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var response FinMindResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return 0, err
	}

	// 尋找最新的股東權益數據
	latestEquity := 0.0
	latestDate := ""

	for _, item := range response.Data {
		// 尋找股東權益相關欄位
		if item.Type == "歸屬於母公司業主之權益合計" || 
		   item.Type == "權益總額" || 
		   item.OriginName == "歸屬於母公司業主之權益合計" ||
		   item.OriginName == "權益總額" {
			if item.Date > latestDate {
				latestDate = item.Date
				latestEquity = item.Value
			}
		}
	}

	if latestEquity == 0 {
		return 0, fmt.Errorf("未找到股東權益數據")
	}

	return latestEquity, nil
}

// GetHistoricalROE 獲取歷史ROE數據 (用於趨勢分析)
func (r *ROECalculator) GetHistoricalROE(stockCode string, years int) ([]float64, error) {
	var historicalROE []float64
	
	for i := 0; i < years; i++ {
		year := time.Now().Year() - i
		startDate := fmt.Sprintf("%d-01-01", year)
		endDate := fmt.Sprintf("%d-12-31", year)
		
		roe, err := r.calculateROEForPeriod(stockCode, startDate, endDate)
		if err != nil {
			fmt.Printf("獲取 %d 年ROE失敗: %v\n", year, err)
			continue
		}
		
		historicalROE = append(historicalROE, roe)
	}
	
	return historicalROE, nil
}

// calculateROEForPeriod 計算特定期間的ROE
func (r *ROECalculator) calculateROEForPeriod(stockCode, startDate, endDate string) (float64, error) {
	// 此處省略具體實現，類似於CalculateROE但指定日期範圍
	// ...
	return 0, nil
}

// 使用範例
func ExampleROEUsage() {
	calculator := NewROECalculator()
	
	// 計算台積電的ROE
	roe, err := calculator.CalculateROE("2330")
	if err != nil {
		fmt.Printf("計算ROE失敗: %v\n", err)
		return
	}
	
	fmt.Printf("台積電ROE: %.2f%%\n", roe)
	
	// 獲取歷史ROE數據
	historicalROE, err := calculator.GetHistoricalROE("2330", 3)
	if err != nil {
		fmt.Printf("獲取歷史ROE失敗: %v\n", err)
		return
	}
	
	fmt.Printf("歷史ROE: %v\n", historicalROE)
}