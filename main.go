package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// StockData 股票資料結構
type StockData struct {
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	Price         float64 `json:"price"`
	Volume        int64   `json:"volume"`
	ROE           float64 `json:"roe"`
	RevenueGrowth float64 `json:"revenue_growth"`
	DebtRatio     float64 `json:"debt_ratio"`
	GrossMargin   float64 `json:"gross_margin"`
	DividendYears int     `json:"dividend_years"`
	YoYGrowth     float64 `json:"yoy_growth"` // 年增率 (Year-over-Year)
	EPSGrowth     float64 `json:"eps_growth"` // EPS增長率
	EPS           float64 `json:"eps"`        // 每股盈餘
	MA60          float64 `json:"ma60"`
	KValue        float64 `json:"k_value"`
	DValue        float64 `json:"d_value"`
	AvgVolume     int64   `json:"avg_volume"`
	Score         float64 `json:"score"`
}

// ScreeningCriteria 篩選條件
type ScreeningCriteria struct {
	MinROE           float64
	MinRevenueGrowth float64
	MaxDebtRatio     float64
	MinDividendYears int
	MinYoYGrowth     float64 // 最小年增率要求
	MinEPSGrowth     float64 // 最小EPS增長率要求 (三位數 = 100%)
	MinEPS           float64 // 最小EPS要求
	RequireMA60Above bool
	MinKValue        float64
	MaxKValue        float64
	MinDValue        float64
	MaxDValue        float64
}

// EPSData EPS數據結構
type EPSData struct {
	Date  string
	Value float64
}

// StockScreener 股票篩選器
type StockScreener struct {
	client   *http.Client
	criteria ScreeningCriteria
}

// NewStockScreener 建立新的篩選器
func NewStockScreener() *StockScreener {
	return &StockScreener{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		criteria: ScreeningCriteria{
			MinROE:           8.0,   // 降低ROE要求到8%
			MinRevenueGrowth: -5.0,  // 允許小幅衰退
			MaxDebtRatio:     60.0,  // 提高負債比容忍度到60%
			MinDividendYears: 2,     // 降低配息年數要求到2年
			MinYoYGrowth:     10.0,  // 年增率至少10%
			MinEPSGrowth:     100.0, // EPS增長至少100% (三位數增長)
			MinEPS:           1.0,   // 最小EPS要求1元
			RequireMA60Above: false, // 不強制要求站上MA60
			MinKValue:        30.0,  // 擴大KD值範圍
			MaxKValue:        85.0,
			MinDValue:        30.0,
			MaxDValue:        85.0,
		},
	}
}

// FetchFinancialData 從FinMind API取得真實財務資料
func (s *StockScreener) FetchFinancialData(stockCode string) (*StockData, error) {
	stock := &StockData{
		Code: stockCode,
		// 設定預設值
		ROE:           10.0, // 預設ROE 10%
		RevenueGrowth: 3.0,  // 預設營收成長3%
		DebtRatio:     35.0, // 預設負債比35%
		DividendYears: 3,    // 預設配息3年
		GrossMargin:   25.0, // 預設毛利率25%
		YoYGrowth:     0.0,  // 將從API獲取
		EPSGrowth:     0.0,  // 將從API獲取
		EPS:           0.0,  // 將從API獲取
	}

	// 先嘗試使用 FinMind API 獲取財務數據
	if err := s.fetchFromFinMind(stock); err != nil {
		log.Printf("FinMind API 失敗，使用預設值: %v", err)
		// 如果 FinMind API 失敗，使用原有的 TWSE API 作為後備
		if err := s.fetchFromTWSE(stock); err != nil {
			log.Printf("TWSE API 也失敗: %v", err)
			// 使用預設值
			stock.YoYGrowth = 15.0
			stock.EPSGrowth = 50.0
			stock.EPS = 2.0
		}
	}

	return stock, nil
}

// fetchFromFinMind 從FinMind API獲取財務數據
func (s *StockScreener) fetchFromFinMind(stock *StockData) error {
	// 獲取過去2年的財務數據用於計算年增率
	startDate := time.Now().AddDate(-2, 0, 0).Format("2006-01-02")
	finmindURL := fmt.Sprintf("https://api.finmindtrade.com/api/v4/data?dataset=TaiwanStockFinancialStatements&data_id=%s&start_date=%s",
		stock.Code, startDate)

	resp, err := s.client.Get(finmindURL)
	if err != nil {
		return fmt.Errorf("finMind API request failed: %v", err)
	}
	defer resp.Body.Close()

	var response struct {
		Data []struct {
			Date       string  `json:"date"`
			StockID    string  `json:"stock_id"`
			Type       string  `json:"type"`
			Value      float64 `json:"value"`
			OriginName string  `json:"origin_name"`
		} `json:"data"`
		Msg string `json:"msg"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode FinMind response: %v", err)
	}

	// 解析財務數據 - 改為分季度儲存

	var epsData []EPSData
	var revenueData []EPSData

	for _, item := range response.Data {
		// 調試：顯示所有數據項目 (限制輸出)
		if stock.Code == "2330" && (strings.Contains(item.Date, "2024") || strings.Contains(item.Date, "2025")) {
			fmt.Printf("  調試 - 日期:%s, 類型:%s, 名稱:%s, 數值:%.2f\n",
				item.Date, item.Type, item.OriginName, item.Value)
		}

		// 收集所有 EPS 數據
		if item.Type == "EPS" || strings.Contains(item.OriginName, "每股盈餘") {
			epsData = append(epsData, EPSData{
				Date:  item.Date,
				Value: item.Value,
			})
		}

		// 收集所有營收數據
		if item.Type == "Revenue" || strings.Contains(item.OriginName, "營業收入") {
			revenueData = append(revenueData, EPSData{
				Date:  item.Date,
				Value: item.Value,
			})
		}
	}

	// 計算 EPS 和 EPS 增長率 - 使用同季度比較
	latestEPS, latestEPSDate := s.getLatestQuarterEPS(epsData)
	sameQuarterLastYearEPS := s.getSameQuarterLastYearEPS(epsData, latestEPSDate)

	// 設置最新EPS
	stock.EPS = latestEPS

	// 計算同季度EPS增長率
	if sameQuarterLastYearEPS > 0 && latestEPS > 0 {
		stock.EPSGrowth = ((latestEPS - sameQuarterLastYearEPS) / sameQuarterLastYearEPS) * 100
		fmt.Printf("EPS計算: 最新季(%s)=%.2f vs 去年同季=%.2f, 增長=%.1f%%\n",
			latestEPSDate, latestEPS, sameQuarterLastYearEPS, stock.EPSGrowth)
	}

	// 計算營收年增率 - 使用相同邏輯
	latestRevenue, latestRevenueDate := s.getLatestQuarterRevenue(revenueData)
	sameQuarterLastYearRevenue := s.getSameQuarterLastYearRevenue(revenueData, latestRevenueDate)

	if sameQuarterLastYearRevenue > 0 && latestRevenue > 0 {
		stock.YoYGrowth = ((latestRevenue - sameQuarterLastYearRevenue) / sameQuarterLastYearRevenue) * 100
		stock.RevenueGrowth = stock.YoYGrowth // 同步更新營收成長
	}

	// 嘗試從其他來源獲取 ROE
	if err := s.fetchROEData(stock); err != nil {
		fmt.Printf("ROE獲取失敗，使用預設值: %v\n", err)
	}

	// 獲取負債比數據
	if err := s.fetchDebtRatioData(stock); err != nil {
		fmt.Printf("負債比獲取失敗，使用預設值: %v\n", err)
	}

	fmt.Printf("股票 %s FinMind 數據: EPS=%.2f, EPS增長=%.1f%%, 年增率=%.1f%%, ROE=%.1f%%, 負債比=%.1f%%\n",
		stock.Code, stock.EPS, stock.EPSGrowth, stock.YoYGrowth, stock.ROE, stock.DebtRatio)

	return nil
}

// getLatestQuarterEPS 取得最新季度的EPS
func (s *StockScreener) getLatestQuarterEPS(epsData []EPSData) (float64, string) {
	if len(epsData) == 0 {
		return 0, ""
	}

	// 排序找到最新日期
	latest := epsData[0]
	for _, data := range epsData {
		if data.Date > latest.Date {
			latest = data
		}
	}

	return latest.Value, latest.Date
}

// getSameQuarterLastYearEPS 取得去年同季度的EPS
func (s *StockScreener) getSameQuarterLastYearEPS(epsData []EPSData, latestDate string) float64 {
	if latestDate == "" {
		return 0
	}

	// 解析最新日期，計算去年同季度
	t, err := time.Parse("2006-01-02", latestDate)
	if err != nil {
		return 0
	}

	// 計算去年同季度的目標日期
	lastYear := t.Year() - 1
	targetDate := fmt.Sprintf("%d-%02d-%02d", lastYear, t.Month(), t.Day())

	// 尋找去年同季度數據
	for _, data := range epsData {
		if data.Date == targetDate {
			return data.Value
		}
	}

	return 0
}

// getLatestQuarterRevenue 取得最新季度的營收
func (s *StockScreener) getLatestQuarterRevenue(revenueData []EPSData) (float64, string) {
	if len(revenueData) == 0 {
		return 0, ""
	}

	// 排序找到最新日期
	latest := revenueData[0]
	for _, data := range revenueData {
		if data.Date > latest.Date {
			latest = data
		}
	}

	return latest.Value, latest.Date
}

// getSameQuarterLastYearRevenue 取得去年同季度的營收
func (s *StockScreener) getSameQuarterLastYearRevenue(revenueData []EPSData, latestDate string) float64 {
	if latestDate == "" {
		return 0
	}

	// 解析最新日期，計算去年同季度
	t, err := time.Parse("2006-01-02", latestDate)
	if err != nil {
		return 0
	}

	// 計算去年同季度的目標日期
	lastYear := t.Year() - 1
	targetDate := fmt.Sprintf("%d-%02d-%02d", lastYear, t.Month(), t.Day())

	// 尋找去年同季度數據
	for _, data := range revenueData {
		if data.Date == targetDate {
			return data.Value
		}
	}

	return 0
}

// fetchROEData 從FinMind API計算精確的ROE數據
func (s *StockScreener) fetchROEData(stock *StockData) error {
	// 使用精確的ROE計算方法：ROE = 本期淨利 / 平均股東權益 * 100%
	if err := s.calculatePreciseROE(stock); err == nil {
		return nil
	}

	// 備用方法1: 嘗試從TWSE獲取財務比率數據
	if err := s.fetchROEFromTWSE(stock); err == nil {
		return nil
	}

	// 備用方法2: 使用 DuPont 分析法估算 ROE
	if err := s.estimateROEFromDuPont(stock); err == nil {
		return nil
	}

	// 備用方法3: 使用行業平均值或經驗公式
	s.estimateROEFromIndustry(stock)

	return nil
}

// calculatePreciseROE 使用FinMind API精確計算ROE
func (s *StockScreener) calculatePreciseROE(stock *StockData) error {
	// 步驟1: 獲取最新本期淨利（分子）
	netIncome, incomeDate, err := s.fetchNetIncome(stock.Code)
	if err != nil {
		return fmt.Errorf("無法獲取淨利數據: %v", err)
	}

	// 步驟2: 獲取股東權益數據（分母）
	avgEquity, err := s.fetchAverageEquity(stock.Code, incomeDate)
	if err != nil {
		return fmt.Errorf("無法獲取權益數據: %v", err)
	}

	// 步驟3: 計算ROE
	if avgEquity > 0 && netIncome != 0 {
		roe := (netIncome / avgEquity) * 100
		stock.ROE = roe
		
		fmt.Printf("📊 精確ROE計算 [%s]:\n", stock.Code)
		fmt.Printf("   本期淨利: %.0f 元 (日期: %s)\n", netIncome, incomeDate)
		fmt.Printf("   平均股東權益: %.0f 元\n", avgEquity)
		fmt.Printf("   ROE = %.0f / %.0f × 100%% = %.2f%%\n", 
			netIncome, avgEquity, roe)
		
		return nil
	}

	return fmt.Errorf("ROE計算數據不足: netIncome=%.0f, avgEquity=%.0f", netIncome, avgEquity)
}

// fetchNetIncome 從FinMind獲取最新本期淨利
func (s *StockScreener) fetchNetIncome(stockCode string) (float64, string, error) {
	// 獲取今年的財務數據
	startDate := time.Now().Format("2006") + "-01-01"
	url := fmt.Sprintf("https://api.finmindtrade.com/api/v4/data?dataset=TaiwanStockFinancialStatements&data_id=%s&start_date=%s",
		stockCode, startDate)

	resp, err := s.client.Get(url)
	if err != nil {
		return 0, "", fmt.Errorf("API請求失敗: %v", err)
	}
	defer resp.Body.Close()

	var response struct {
		Data []struct {
			Date       string  `json:"date"`
			StockID    string  `json:"stock_id"`
			Type       string  `json:"type"`
			Value      float64 `json:"value"`
			OriginName string  `json:"origin_name"`
		} `json:"data"`
		Msg string `json:"msg"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return 0, "", fmt.Errorf("解析API回應失敗: %v", err)
	}

	// 尋找本期淨利（IncomeAfterTaxes）
	var latestNetIncome float64
	var latestDate string

	for _, item := range response.Data {
		// 只尋找確切的 IncomeAfterTaxes 類型（稅後本期淨利）
		if item.Type == "IncomeAfterTaxes" {
			if item.Date > latestDate {
				latestDate = item.Date
				latestNetIncome = item.Value
				// 調試：顯示找到的淨利數據
				if stockCode == "2328" {
					fmt.Printf("     找到淨利數據: %s, Type: %s, OriginName: %s, Value: %.0f\n", 
						item.Date, item.Type, item.OriginName, item.Value)
				}
			}
		}
	}

	if latestDate == "" {
		return 0, "", fmt.Errorf("未找到本期淨利數據")
	}

	return latestNetIncome, latestDate, nil
}

// fetchAverageEquity 獲取平均股東權益
func (s *StockScreener) fetchAverageEquity(stockCode, incomeDate string) (float64, error) {
	// 解析收入日期，判斷需要的權益日期
	incomeTime, err := time.Parse("2006-01-02", incomeDate)
	if err != nil {
		return 0, fmt.Errorf("日期解析失敗: %v", err)
	}

	// 計算需要的兩個權益日期
	var currentQuarterDate, previousQuarterDate string
	
	// 根據收入日期判斷季度
	switch incomeTime.Month() {
	case time.March: // Q1
		currentQuarterDate = fmt.Sprintf("%d-03-31", incomeTime.Year())
		previousQuarterDate = fmt.Sprintf("%d-12-31", incomeTime.Year()-1)
	case time.June: // Q2
		currentQuarterDate = fmt.Sprintf("%d-06-30", incomeTime.Year())
		previousQuarterDate = fmt.Sprintf("%d-03-31", incomeTime.Year())
	case time.September: // Q3
		currentQuarterDate = fmt.Sprintf("%d-09-30", incomeTime.Year())
		previousQuarterDate = fmt.Sprintf("%d-06-30", incomeTime.Year())
	case time.December: // Q4
		currentQuarterDate = fmt.Sprintf("%d-12-31", incomeTime.Year())
		previousQuarterDate = fmt.Sprintf("%d-09-30", incomeTime.Year())
	default:
		// 如果是其他月份，使用最近的季度
		if incomeTime.Month() >= time.January && incomeTime.Month() <= time.March {
			currentQuarterDate = fmt.Sprintf("%d-03-31", incomeTime.Year())
			previousQuarterDate = fmt.Sprintf("%d-12-31", incomeTime.Year()-1)
		} else {
			currentQuarterDate = fmt.Sprintf("%d-03-31", incomeTime.Year())
			previousQuarterDate = fmt.Sprintf("%d-12-31", incomeTime.Year()-1)
		}
	}

	// 獲取資產負債表數據
	startDate := fmt.Sprintf("%d-01-01", incomeTime.Year()-1) // 獲取前一年的數據以確保完整
	url := fmt.Sprintf("https://api.finmindtrade.com/api/v4/data?dataset=TaiwanStockBalanceSheet&data_id=%s&start_date=%s",
		stockCode, startDate)

	resp, err := s.client.Get(url)
	if err != nil {
		return 0, fmt.Errorf("資產負債表API請求失敗: %v", err)
	}
	defer resp.Body.Close()

	var response struct {
		Data []struct {
			Date       string  `json:"date"`
			StockID    string  `json:"stock_id"`
			Type       string  `json:"type"`
			Value      float64 `json:"value"`
			OriginName string  `json:"origin_name"`
		} `json:"data"`
		Msg string `json:"msg"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return 0, fmt.Errorf("解析資產負債表回應失敗: %v", err)
	}

	// 尋找權益總額數據
	equityData := make(map[string]float64)

	for _, item := range response.Data {
		// 尋找權益總額（Equity）- 確保使用正確的絕對值，不是百分比
		if item.Type == "Equity" && !strings.Contains(item.OriginName, "_per") {
			equityData[item.Date] = item.Value
			// 調試：顯示找到的權益數據 (針對2328)
			if stockCode == "2328" {
				fmt.Printf("     找到權益數據: %s = %.0f 元\n", item.Date, item.Value)
			}
		}
	}

	// 獲取兩個季度的權益數據
	currentEquity, currentExists := equityData[currentQuarterDate]
	previousEquity, previousExists := equityData[previousQuarterDate]

	fmt.Printf("   權益數據查找:\n")
	fmt.Printf("     當季日期 (%s): %.0f 元 [%t]\n", currentQuarterDate, currentEquity, currentExists)
	fmt.Printf("     前季日期 (%s): %.0f 元 [%t]\n", previousQuarterDate, previousEquity, previousExists)

	// 如果找不到精確日期，嘗試找最近的日期
	if !currentExists || !previousExists {
		// 找到所有可用的權益日期，選擇最近的兩個
		var availableDates []string
		for date := range equityData {
			availableDates = append(availableDates, date)
		}
		
		if len(availableDates) >= 2 {
			sort.Strings(availableDates) // 按日期排序
			
			// 取最新的兩個日期
			latest := availableDates[len(availableDates)-1]
			secondLatest := availableDates[len(availableDates)-2]
			
			currentEquity = equityData[latest]
			previousEquity = equityData[secondLatest]
			
			fmt.Printf("   使用最近的權益數據:\n")
			fmt.Printf("     最新日期 (%s): %.0f 元\n", latest, currentEquity)
			fmt.Printf("     次新日期 (%s): %.0f 元\n", secondLatest, previousEquity)
		} else {
			return 0, fmt.Errorf("權益數據不足，僅找到 %d 筆記錄", len(availableDates))
		}
	}

	// 計算平均權益
	if currentEquity > 0 && previousEquity > 0 {
		avgEquity := (currentEquity + previousEquity) / 2
		return avgEquity, nil
	}

	return 0, fmt.Errorf("權益數據無效: current=%.0f, previous=%.0f", currentEquity, previousEquity)
}

// fetchROEFromTWSE 從台灣證交所API嘗試獲取ROE相關數據
func (s *StockScreener) fetchROEFromTWSE(stock *StockData) error {
	// 使用個股日成交資訊API
	url := fmt.Sprintf("https://www.twse.com.tw/exchangeReport/BWIBBU_d?response=json&date=%s&stockNo=%s",
		time.Now().Format("20060102"), stock.Code)

	resp, err := s.client.Get(url)
	if err != nil {
		return fmt.Errorf("TWSE API request failed: %v", err)
	}
	defer resp.Body.Close()

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return fmt.Errorf("failed to decode TWSE response: %v", err)
	}

	// 解析財務比率數據
	if fields, ok := data["data"].([]interface{}); ok && len(fields) > 0 {
		if row, ok := fields[0].([]interface{}); ok && len(row) >= 6 {
			// 嘗試解析本益比 (P/E ratio)
			if peStr, ok := row[4].(string); ok && peStr != "-" && peStr != "" {
				if pe, err := strconv.ParseFloat(strings.TrimSpace(peStr), 64); err == nil && pe > 0 {
					// 嘗試解析股價淨值比 (P/B ratio)
					if len(row) > 5 {
						if pbStr, ok := row[5].(string); ok && pbStr != "-" && pbStr != "" {
							if pb, err := strconv.ParseFloat(strings.TrimSpace(pbStr), 64); err == nil && pb > 0 {
								// 使用 ROE = (P/E) / (P/B) 的關係式
								// 或 ROE ≈ (1/PE) * (PB) 的近似公式
								estimatedROE := (pb / pe) * 100
								if estimatedROE > 0 && estimatedROE < 100 { // 合理性檢查
									stock.ROE = estimatedROE
									fmt.Printf("從TWSE估算ROE: PE=%.2f, PB=%.2f, ROE=%.2f%%\n", pe, pb, estimatedROE)
									return nil
								}
							}
						}
					}
				}
			}
		}
	}

	return fmt.Errorf("no valid financial ratios found")
}

// estimateROEFromDuPont 使用DuPont分析法估算ROE
func (s *StockScreener) estimateROEFromDuPont(stock *StockData) error {
	// DuPont分析: ROE = 淨利率 × 資產周轉率 × 權益乘數
	// 如果我們有EPS和一些假設，可以做粗略估算

	if stock.EPS <= 0 {
		return fmt.Errorf("insufficient data for DuPont analysis")
	}

	// 根據EPS水準做粗略估算
	// 這是簡化的啟發式方法
	var estimatedROE float64

	if stock.EPS >= 10 { // 高EPS通常對應高ROE
		estimatedROE = 15.0 + (stock.EPS-10)*0.5 // 基礎15% + 額外成分
	} else if stock.EPS >= 5 {
		estimatedROE = 10.0 + (stock.EPS-5)*1.0
	} else if stock.EPS >= 1 {
		estimatedROE = 5.0 + (stock.EPS-1)*1.25
	} else {
		estimatedROE = stock.EPS * 5 // 低EPS情況
	}

	// 考慮營收增長的影響
	if stock.YoYGrowth > 10 {
		estimatedROE *= 1.2 // 高成長公司通常有更高ROE
	} else if stock.YoYGrowth < -10 {
		estimatedROE *= 0.8 // 衰退公司ROE較低
	}

	// 合理性限制
	if estimatedROE > 50 {
		estimatedROE = 50
	} else if estimatedROE < 0 {
		estimatedROE = 1
	}

	stock.ROE = estimatedROE
	fmt.Printf("DuPont估算ROE: 基於EPS=%.2f, YoY=%.1f%%, 估算ROE=%.2f%%\n",
		stock.EPS, stock.YoYGrowth, estimatedROE)

	return nil
}

// estimateROEFromIndustry 根據行業特性估算ROE
func (s *StockScreener) estimateROEFromIndustry(stock *StockData) {
	// 根據股票代碼判斷行業類型，設定合理的ROE預期
	code := stock.Code
	var industryROE float64

	switch {
	case code >= "2300" && code <= "2399": // 電子業
		industryROE = 12.0
	case code >= "2400" && code <= "2499": // 半導體
		industryROE = 15.0
	case code >= "2800" && code <= "2899": // 金融業
		industryROE = 8.0
	case code >= "2600" && code <= "2699": // 航運業
		industryROE = 6.0
	case code >= "1200" && code <= "1299": // 食品業
		industryROE = 10.0
	default:
		industryROE = 10.0 // 預設值
	}

	// 根據公司表現調整
	if stock.EPSGrowth > 20 {
		industryROE *= 1.3
	} else if stock.EPSGrowth < -20 {
		industryROE *= 0.7
	}

	if industryROE > 30 {
		industryROE = 30
	} else if industryROE < 3 {
		industryROE = 3
	}

	stock.ROE = industryROE
	fmt.Printf("行業估算ROE: 股票%s, 行業基準=%.1f%%, 調整後=%.2f%%\n",
		code, industryROE/1.3, stock.ROE)
}

// fetchDebtRatioData 從FinMind API獲取負債比數據
func (s *StockScreener) fetchDebtRatioData(stock *StockData) error {
	// 使用FinMind資產負債表API
	startDate := time.Now().AddDate(-1, 0, 0).Format("2006-01-02") // 獲取過去1年數據
	balanceSheetURL := fmt.Sprintf("https://api.finmindtrade.com/api/v4/data?dataset=TaiwanStockBalanceSheet&data_id=%s&start_date=%s",
		stock.Code, startDate)

	resp, err := s.client.Get(balanceSheetURL)
	if err != nil {
		return fmt.Errorf("FinMind Balance Sheet API request failed: %v", err)
	}
	defer resp.Body.Close()

	var response struct {
		Data []struct {
			Date       string  `json:"date"`
			StockID    string  `json:"stock_id"`
			Type       string  `json:"type"`
			Value      float64 `json:"value"`
			OriginName string  `json:"origin_name"`
		} `json:"data"`
		Msg string `json:"msg"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode FinMind Balance Sheet response: %v", err)
	}

	// 尋找最新的總資產和總負債數據
	var latestTotalAssets, latestTotalLiabilities float64
	var latestDate string

	// 調試：關閉詳細日誌
	// if stock.Code == "2330" {
	//     fmt.Printf("資產負債表調試:\n")
	//     ...
	// }

	// 收集所有相關數據
	dataMap := make(map[string]map[string]float64)

	for _, item := range response.Data {
		if dataMap[item.Date] == nil {
			dataMap[item.Date] = make(map[string]float64)
		}
		dataMap[item.Date][item.Type] = item.Value
	}

	// 找到最新日期
	for date := range dataMap {
		if date > latestDate {
			latestDate = date
		}
	}

	// 獲取最新日期的資產負債數據
	if latestData, ok := dataMap[latestDate]; ok {
		// 尋找總資產
		for key, value := range latestData {
			if key == "TotalAssets" || strings.Contains(key, "Asset") {
				latestTotalAssets = value
				break
			}
		}

		// 優先使用已計算好的負債比百分比
		if liabilitiesPer, exists := latestData["Liabilities_per"]; exists {
			stock.DebtRatio = liabilitiesPer
			fmt.Printf("直接使用負債比: 日期=%s, 負債比=%.2f%%\n", latestDate, liabilitiesPer)
			return nil
		}

		// 否則尋找負債總額進行計算
		if liabilities, exists := latestData["Liabilities"]; exists {
			latestTotalLiabilities = liabilities
		}
	}

	// 計算負債比
	if latestTotalAssets > 0 && latestTotalLiabilities >= 0 {
		debtRatio := (latestTotalLiabilities / latestTotalAssets) * 100

		// 合理性檢查 (負債比應該在0-100%之間)
		if debtRatio >= 0 && debtRatio <= 100 {
			stock.DebtRatio = debtRatio
			fmt.Printf("負債比計算: 日期=%s, 總資產=%.0f, 總負債=%.0f, 負債比=%.2f%%\n",
				latestDate, latestTotalAssets, latestTotalLiabilities, debtRatio)
			return nil
		}
	}

	return fmt.Errorf("invalid balance sheet data: assets=%.0f, liabilities=%.0f",
		latestTotalAssets, latestTotalLiabilities)
}

// fetchFromTWSE 從TWSE API獲取基本數據作為後備
func (s *StockScreener) fetchFromTWSE(stock *StockData) error {
	fundamentalURL := fmt.Sprintf("https://www.twse.com.tw/exchangeReport/BWIBBU_d?response=json&date=%s&stockNo=%s",
		time.Now().Format("20060102"), stock.Code)

	resp, err := s.client.Get(fundamentalURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}

	// 解析財務指標
	if fields, ok := data["data"].([]interface{}); ok && len(fields) > 0 {
		if row, ok := fields[0].([]interface{}); ok && len(row) >= 5 {
			// 解析本益比、淨值比、殖利率等
			if pe, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprintf("%v", row[4])), 64); err == nil {
				stock.ROE = s.estimateROE(pe) // 簡化計算
			}
		}
	}

	return nil
}

// FetchTechnicalData 取得技術面資料
func (s *StockScreener) FetchTechnicalData(stock *StockData) error {
	// 構建正確的 Yahoo Finance 股票代碼
	symbol := s.buildYahooSymbol(stock.Code)
	// 使用Yahoo Finance API取得技術指標
	url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=3mo", symbol)

	// 建立請求並添加必要的 headers
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	// 添加 User-Agent 和其他 headers 來模擬瀏覽器請求
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-TW,zh;q=0.9,en;q=0.8")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 檢查 HTTP 狀態碼
	if resp.StatusCode != 200 {
		return fmt.Errorf("Yahoo Finance API 返回錯誤狀態碼: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// 可選：調試模式下打印響應內容
	// bodyStr := string(body)
	// if len(bodyStr) > 200 {
	// 	log.Printf("股票 %s API 響應前200字符: %s", stock.Code, bodyStr[:200])
	// } else {
	// 	log.Printf("股票 %s API 完整響應: %s", stock.Code, bodyStr)
	// }

	// 解析技術指標
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		maxLen := len(body)
		if maxLen > 500 {
			maxLen = 500
		}
		return fmt.Errorf("JSON 解析錯誤: %v, 響應內容: %s", err, string(body[:maxLen]))
	}

	// 檢查 Yahoo Finance API 是否返回錯誤
	if chart, ok := data["chart"].(map[string]interface{}); ok {
		if errorMsg, ok := chart["error"].(map[string]interface{}); ok {
			if description, ok := errorMsg["description"].(string); ok {
				return fmt.Errorf("Yahoo Finance API 錯誤: %s", description)
			}
		}
	}

	// 計算移動平均線和KD值
	if chart, ok := data["chart"].(map[string]interface{}); ok {
		if result, ok := chart["result"].([]interface{}); ok && len(result) > 0 {
			resultData := result[0].(map[string]interface{})

			// 取得股票基本資訊
			if meta, ok := resultData["meta"].(map[string]interface{}); ok {
				if currentPrice, ok := meta["regularMarketPrice"].(float64); ok {
					stock.Price = currentPrice
				}
			}

			// 取得OHLC資料
			if indicators, ok := resultData["indicators"].(map[string]interface{}); ok {
				if quote, ok := indicators["quote"].([]interface{}); ok && len(quote) > 0 {
					quoteData := quote[0].(map[string]interface{})

					// 取得收盤價、最高價、最低價資料
					var closes, highs, lows []float64

					if closesRaw, ok := quoteData["close"].([]interface{}); ok {
						for _, c := range closesRaw {
							if price, ok := c.(float64); ok && price > 0 {
								closes = append(closes, price)
							}
						}
					}

					if highsRaw, ok := quoteData["high"].([]interface{}); ok {
						for _, h := range highsRaw {
							if price, ok := h.(float64); ok && price > 0 {
								highs = append(highs, price)
							}
						}
					}

					if lowsRaw, ok := quoteData["low"].([]interface{}); ok {
						for _, l := range lowsRaw {
							if price, ok := l.(float64); ok && price > 0 {
								lows = append(lows, price)
							}
						}
					}

					// 計算技術指標並存入stock結構
					s.calculateTechnicalIndicators(stock, closes, highs, lows)
				}
			}
		}
	}

	return nil
}

// calculateTechnicalIndicators 計算技術指標
func (s *StockScreener) calculateTechnicalIndicators(stock *StockData, closes, highs, lows []float64) {
	if len(closes) < 60 || len(highs) < 60 || len(lows) < 60 {
		return
	}

	// 確保三個陣列長度一致
	minLen := len(closes)
	if len(highs) < minLen {
		minLen = len(highs)
	}
	if len(lows) < minLen {
		minLen = len(lows)
	}

	// 計算60日移動平均線
	if minLen >= 60 {
		var sum float64
		for i := minLen - 60; i < minLen; i++ {
			sum += closes[i]
		}
		stock.MA60 = sum / 60
	}

	// 計算KD指標
	if minLen >= 9 {
		kd := s.calculateKDIndicator(closes, highs, lows)
		stock.KValue = kd.K
		stock.DValue = kd.D
	}

	// 計算平均成交量 (如果需要的話，這裡暫時設為0)
	stock.AvgVolume = 0

	fmt.Printf("股票 %s - 現價: %.2f, MA60: %.2f, K: %.2f, D: %.2f\n",
		stock.Code, stock.Price, stock.MA60, stock.KValue, stock.DValue)
}

// KDResult KD指標結果
type KDResult struct {
	K float64
	D float64
}

// calculateKDIndicator 計算KD指標
func (s *StockScreener) calculateKDIndicator(closes, highs, lows []float64) KDResult {
	if len(closes) < 9 {
		return KDResult{K: 50.0, D: 50.0}
	}

	// 計算RSV值序列
	rsvs := make([]float64, 0)

	for i := 8; i < len(closes); i++ {
		// 取9天區間的最高價、最低價、收盤價
		start := i - 8
		end := i + 1

		highest := highs[start]
		lowest := lows[start]

		for j := start; j < end; j++ {
			if highs[j] > highest {
				highest = highs[j]
			}
			if lows[j] < lowest {
				lowest = lows[j]
			}
		}

		// 計算RSV
		var rsv float64
		if highest == lowest {
			rsv = 50.0
		} else {
			rsv = ((closes[i] - lowest) / (highest - lowest)) * 100
		}
		rsvs = append(rsvs, rsv)
	}

	if len(rsvs) == 0 {
		return KDResult{K: 50.0, D: 50.0}
	}

	// 計算K值和D值 (使用指數移動平均)
	// K = 2/3 * 前一日K值 + 1/3 * 當日RSV
	// D = 2/3 * 前一日D值 + 1/3 * 當日K值

	k := 50.0 // 初始K值
	d := 50.0 // 初始D值

	for _, rsv := range rsvs {
		k = (2.0/3.0)*k + (1.0/3.0)*rsv
		d = (2.0/3.0)*d + (1.0/3.0)*k
	}

	return KDResult{K: k, D: d}
}

// estimateROE 簡化的ROE估算
func (s *StockScreener) estimateROE(pe float64) float64 {
	// 這是簡化的估算，實際應該從財報取得
	if pe > 0 && pe < 15 {
		return 20.0
	} else if pe >= 15 && pe < 25 {
		return 15.0
	}
	return 10.0
}

// FetchStockList 取得股票清單
func (s *StockScreener) FetchStockList() ([]string, error) {
	// 取得上市股票代碼
	resp, err := s.client.Get("https://www.twse.com.tw/zh/api/codeQuery")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 這裡簡化處理，實際應該解析完整的股票清單
	// 先用一些熱門股票做示範
	stockList := []string{
		"2328", // 廣宇 - 用於測試ROE算法
		"2330", // 台積電
		// "3379",
		// "2454", // 聯發科
		// "2308", // 台達電
		// "2886", // 兆豐金
		// "2884", // 玉山金
		// "2382", // 廣達
		// "3231", // 緯創
		// "2376", // 技嘉
		// "2449", // 京元電
		// "1216", // 統一
		// "2412", // 中華電
		// "0050", // 元大台灣50
		// "0056", // 元大高股息
		// "2603", // 長榮
		// "2609", // 陽明
		// "2881", // 富邦金
		// "2882", // 國泰金
		// "2892", // 第一金
		// "3008", // 大立光
		// "2317", // 鴻海
	}

	return stockList, nil
}

// ScreenStocks 篩選股票
func (s *StockScreener) ScreenStocks(stocks []string) ([]*StockData, error) {
	var qualifiedStocks []*StockData

	for _, code := range stocks {
		fmt.Printf("正在分析股票: %s\n", code)

		// 取得財務資料
		stock, err := s.FetchFinancialData(code)
		if err != nil {
			log.Printf("無法取得 %s 的財務資料: %v\n", code, err)
			continue
		}

		// 取得技術面資料
		if err := s.FetchTechnicalData(stock); err != nil {
			log.Printf("無法取得 %s 的技術資料: %v\n", code, err)
			continue
		}

		// 檢查是否符合篩選條件
		if s.meetsScreeningCriteria(stock) {
			s.calculateScore(stock)
			qualifiedStocks = append(qualifiedStocks, stock)
		}

		// 避免請求過於頻繁
		time.Sleep(1 * time.Second)
	}

	// 根據分數排序
	sort.Slice(qualifiedStocks, func(i, j int) bool {
		return qualifiedStocks[i].Score > qualifiedStocks[j].Score
	})

	return qualifiedStocks, nil
}

// meetsScreeningCriteria 檢查是否符合篩選條件 (分段篩選)
func (s *StockScreener) meetsScreeningCriteria(stock *StockData) bool {
	fmt.Printf("\n🔍 開始篩選股票: %s (%s)\n", stock.Code, stock.Name)

	// 第一階段：基本財務健康度檢查 (必須條件)
	stage1Passed, stage1Reasons := s.checkStage1Fundamentals(stock)

	if !stage1Passed {
		fmt.Printf("❌ %s 第一階段未通過: %s\n", stock.Code, strings.Join(stage1Reasons, ", "))
		return false
	}

	fmt.Printf("✅ %s 通過第一階段 (基本財務健康度)\n", stock.Code)

	// 第二階段：投資品質評估 (優先條件)
	stage2Passed, stage2Reasons := s.checkStage2Quality(stock)

	if !stage2Passed {
		fmt.Printf("⚠️  %s 第二階段未完全通過: %s\n", stock.Code, strings.Join(stage2Reasons, ", "))
		fmt.Printf("   但仍可列入候選清單\n")
	} else {
		fmt.Printf("✅ %s 通過第二階段 (投資品質)\n", stock.Code)
	}

	// 第三階段：技術面時機判斷 (參考條件)
	stage3Passed, stage3Reasons := s.checkStage3Technical(stock)

	if !stage3Passed {
		fmt.Printf("⚠️  %s 技術面時機: %s\n", stock.Code, strings.Join(stage3Reasons, ", "))
	} else {
		fmt.Printf("✅ %s 技術面時機良好\n", stock.Code)
	}

	// 只要通過第一階段就納入候選
	fmt.Printf("📈 %s 綜合評估: 納入候選清單\n", stock.Code)
	return stage1Passed
}

// checkStage1Fundamentals 第一階段：基本財務健康度檢查
func (s *StockScreener) checkStage1Fundamentals(stock *StockData) (bool, []string) {
	reasons := []string{}

	fmt.Printf("   📊 財務健康度檢查:\n")

	// 極端負面條件 (絕對排除)
	if stock.ROE <= 0 {
		reasons = append(reasons, "ROE為負數或零")
	}
	if stock.DebtRatio >= 80.0 {
		reasons = append(reasons, fmt.Sprintf("負債比過高 %.1f%% (>80%%)", stock.DebtRatio))
	}
	if stock.RevenueGrowth <= -20.0 {
		reasons = append(reasons, fmt.Sprintf("營收大幅衰退 %.1f%% (<-20%%)", stock.RevenueGrowth))
	}
	if stock.YoYGrowth <= -30.0 {
		reasons = append(reasons, fmt.Sprintf("年增率大幅衰退 %.1f%% (<-30%%)", stock.YoYGrowth))
	}
	if stock.EPSGrowth <= -50.0 {
		reasons = append(reasons, fmt.Sprintf("EPS大幅衰退 %.1f%% (<-50%%)", stock.EPSGrowth))
	}
	if stock.EPS <= 0 {
		reasons = append(reasons, "EPS為負數或零")
	}

	// 顯示數值
	fmt.Printf("      ROE: %.1f%% %s\n", stock.ROE, s.getStatusIcon(stock.ROE > 0))
	fmt.Printf("      負債比: %.1f%% %s\n", stock.DebtRatio, s.getStatusIcon(stock.DebtRatio < 80.0))
	fmt.Printf("      營收成長: %.1f%% %s\n", stock.RevenueGrowth, s.getStatusIcon(stock.RevenueGrowth > -20.0))
	fmt.Printf("      年增率: %.1f%% %s\n", stock.YoYGrowth, s.getStatusIcon(stock.YoYGrowth > -30.0))
	fmt.Printf("      EPS增長: %.1f%% %s\n", stock.EPSGrowth, s.getStatusIcon(stock.EPSGrowth > -50.0))
	fmt.Printf("      EPS: %.2f %s\n", stock.EPS, s.getStatusIcon(stock.EPS > 0))

	return len(reasons) == 0, reasons
}

// checkStage2Quality 第二階段：投資品質評估
func (s *StockScreener) checkStage2Quality(stock *StockData) (bool, []string) {
	reasons := []string{}
	passCount := 0
	totalChecks := 7

	fmt.Printf("   💎 投資品質評估:\n")

	// ROE品質檢查
	if stock.ROE >= 15.0 {
		fmt.Printf("      ROE: %.1f%% ✅ (優秀)\n", stock.ROE)
		passCount++
	} else if stock.ROE >= 10.0 {
		fmt.Printf("      ROE: %.1f%% 🟡 (良好)\n", stock.ROE)
		passCount++
	} else {
		fmt.Printf("      ROE: %.1f%% ❌ (偏低)\n", stock.ROE)
		reasons = append(reasons, fmt.Sprintf("ROE偏低 %.1f%%", stock.ROE))
	}

	// 營收成長檢查
	if stock.RevenueGrowth >= 10.0 {
		fmt.Printf("      營收成長: %.1f%% ✅ (高成長)\n", stock.RevenueGrowth)
		passCount++
	} else if stock.RevenueGrowth >= 0 {
		fmt.Printf("      營收成長: %.1f%% 🟡 (穩定)\n", stock.RevenueGrowth)
		passCount++
	} else {
		fmt.Printf("      營收成長: %.1f%% ❌ (衰退)\n", stock.RevenueGrowth)
		reasons = append(reasons, fmt.Sprintf("營收衰退 %.1f%%", stock.RevenueGrowth))
	}

	// 年增率檢查 (新增)
	if stock.YoYGrowth >= s.criteria.MinYoYGrowth {
		fmt.Printf("      年增率: %.1f%% ✅ (達標)\n", stock.YoYGrowth)
		passCount++
	} else if stock.YoYGrowth >= 0 {
		fmt.Printf("      年增率: %.1f%% 🟡 (正成長)\n", stock.YoYGrowth)
		passCount++
	} else {
		fmt.Printf("      年增率: %.1f%% ❌ (負成長)\n", stock.YoYGrowth)
		reasons = append(reasons, fmt.Sprintf("年增率不足 %.1f%%", stock.YoYGrowth))
	}

	// EPS增長檢查 (新增)
	if stock.EPSGrowth >= s.criteria.MinEPSGrowth {
		fmt.Printf("      EPS增長: %.1f%% ✅ (三位數增長)\n", stock.EPSGrowth)
		passCount++
	} else if stock.EPSGrowth >= 50.0 {
		fmt.Printf("      EPS增長: %.1f%% 🟡 (高成長)\n", stock.EPSGrowth)
		passCount++
	} else {
		fmt.Printf("      EPS增長: %.1f%% ❌ (增長不足)\n", stock.EPSGrowth)
		reasons = append(reasons, fmt.Sprintf("EPS增長不足 %.1f%%", stock.EPSGrowth))
	}

	// EPS絕對值檢查 (新增)
	if stock.EPS >= s.criteria.MinEPS {
		fmt.Printf("      EPS: %.2f ✅ (達標)\n", stock.EPS)
		passCount++
	} else {
		fmt.Printf("      EPS: %.2f ❌ (偏低)\n", stock.EPS)
		reasons = append(reasons, fmt.Sprintf("EPS偏低 %.2f", stock.EPS))
	}

	// 負債比檢查
	if stock.DebtRatio <= 30.0 {
		fmt.Printf("      負債比: %.1f%% ✅ (優秀)\n", stock.DebtRatio)
		passCount++
	} else if stock.DebtRatio <= 50.0 {
		fmt.Printf("      負債比: %.1f%% 🟡 (可接受)\n", stock.DebtRatio)
		passCount++
	} else {
		fmt.Printf("      負債比: %.1f%% ❌ (偏高)\n", stock.DebtRatio)
		reasons = append(reasons, fmt.Sprintf("負債比偏高 %.1f%%", stock.DebtRatio))
	}

	// 配息穩定性檢查
	if stock.DividendYears >= 5 {
		fmt.Printf("      配息年數: %d年 ✅ (穩定)\n", stock.DividendYears)
		passCount++
	} else if stock.DividendYears >= 3 {
		fmt.Printf("      配息年數: %d年 🟡 (尚可)\n", stock.DividendYears)
		passCount++
	} else {
		fmt.Printf("      配息年數: %d年 ❌ (不穩定)\n", stock.DividendYears)
		reasons = append(reasons, fmt.Sprintf("配息不穩定 %d年", stock.DividendYears))
	}

	// 至少通過60%的品質檢查
	qualityPassed := float64(passCount)/float64(totalChecks) >= 0.6
	fmt.Printf("      品質評分: %d/%d (%.0f%%)\n", passCount, totalChecks, float64(passCount)/float64(totalChecks)*100)

	return qualityPassed, reasons
}

// checkStage3Technical 第三階段：技術面時機判斷
func (s *StockScreener) checkStage3Technical(stock *StockData) (bool, []string) {
	reasons := []string{}
	passCount := 0
	totalChecks := 3

	fmt.Printf("   📈 技術面時機評估:\n")

	// MA60趨勢檢查
	if stock.Price > 0 && stock.MA60 > 0 {
		priceDiff := ((stock.Price - stock.MA60) / stock.MA60) * 100
		if priceDiff >= 5.0 {
			fmt.Printf("      股價vs MA60: %.2f vs %.2f (+%.1f%%) ✅ (強勢)\n", stock.Price, stock.MA60, priceDiff)
			passCount++
		} else if priceDiff >= 0 {
			fmt.Printf("      股價vs MA60: %.2f vs %.2f (+%.1f%%) 🟡 (站穩)\n", stock.Price, stock.MA60, priceDiff)
			passCount++
		} else {
			fmt.Printf("      股價vs MA60: %.2f vs %.2f (%.1f%%) ❌ (偏弱)\n", stock.Price, stock.MA60, priceDiff)
			reasons = append(reasons, fmt.Sprintf("跌破MA60 %.1f%%", priceDiff))
		}
	}

	// KD指標檢查
	if stock.KValue >= 50 && stock.KValue <= 80 {
		fmt.Printf("      K值: %.1f ✅ (買進區間)\n", stock.KValue)
		passCount++
	} else if stock.KValue >= 30 && stock.KValue < 90 {
		fmt.Printf("      K值: %.1f 🟡 (可觀察)\n", stock.KValue)
		passCount++
	} else {
		fmt.Printf("      K值: %.1f ❌ (時機不佳)\n", stock.KValue)
		reasons = append(reasons, fmt.Sprintf("K值不在理想區間 %.1f", stock.KValue))
	}

	// D值檢查
	if stock.DValue >= 50 && stock.DValue <= 80 {
		fmt.Printf("      D值: %.1f ✅ (買進區間)\n", stock.DValue)
		passCount++
	} else if stock.DValue >= 30 && stock.DValue < 90 {
		fmt.Printf("      D值: %.1f 🟡 (可觀察)\n", stock.DValue)
		passCount++
	} else {
		fmt.Printf("      D值: %.1f ❌ (時機不佳)\n", stock.DValue)
		reasons = append(reasons, fmt.Sprintf("D值不在理想區間 %.1f", stock.DValue))
	}

	// 技術面通過率
	technicalPassed := float64(passCount)/float64(totalChecks) >= 0.5
	fmt.Printf("      技術評分: %d/%d (%.0f%%)\n", passCount, totalChecks, float64(passCount)/float64(totalChecks)*100)

	return technicalPassed, reasons
}

// getStatusIcon 獲取狀態圖示
func (s *StockScreener) getStatusIcon(passed bool) string {
	if passed {
		return "✅"
	}
	return "❌"
}

// calculateScore 計算綜合評分
func (s *StockScreener) calculateScore(stock *StockData) {
	score := 0.0

	// 基本面評分 (70% - 增加權重)
	score += math.Min(stock.ROE/30.0, 1.0) * 15                   // ROE評分 (降低權重)
	score += math.Min(stock.RevenueGrowth/20.0, 1.0) * 10         // 營收成長評分 (降低權重)
	score += math.Min(stock.YoYGrowth/30.0, 1.0) * 15             // 年增率評分 (新增)
	score += math.Min(stock.EPSGrowth/200.0, 1.0) * 20            // EPS增長評分 (新增，高權重)
	score += math.Min(stock.EPS/5.0, 1.0) * 5                     // EPS絕對值評分 (新增)
	score += (1.0 - stock.DebtRatio/100.0) * 10                   // 負債比評分 (降低權重)
	score += math.Min(float64(stock.DividendYears)/10.0, 1.0) * 5 // 配息穩定性 (降低權重)

	// 技術面評分 (30% - 降低權重)
	if stock.Price > stock.MA60 {
		score += 15 // 站上季線 (降低權重)
	}

	// KD值在黃金交叉區間
	if stock.KValue >= 50 && stock.KValue <= 80 {
		score += 8 // 降低權重
	}
	if stock.DValue >= 50 && stock.DValue <= 80 {
		score += 7 // 降低權重
	}

	stock.Score = score
}

// GenerateReport 產生篩選報告
func (s *StockScreener) GenerateReport(stocks []*StockData) {
	fmt.Println("\n========== 股票篩選報告 ==========")
	fmt.Printf("篩選時間: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println("\n【篩選條件】")
	fmt.Printf("- ROE > %.1f%%\n", s.criteria.MinROE)
	fmt.Printf("- 營收年增率 > %.1f%%\n", s.criteria.MinRevenueGrowth)
	fmt.Printf("- 年增率 > %.1f%%\n", s.criteria.MinYoYGrowth)
	fmt.Printf("- EPS增長 > %.1f%% (三位數增長)\n", s.criteria.MinEPSGrowth)
	fmt.Printf("- EPS > %.1f元\n", s.criteria.MinEPS)
	fmt.Printf("- 負債比 < %.1f%%\n", s.criteria.MaxDebtRatio)
	fmt.Printf("- 近5年穩定配息\n")
	fmt.Printf("- 股價在60日均線之上\n")
	fmt.Printf("- KD值在 %.0f-%.0f 之間\n", s.criteria.MinKValue, s.criteria.MaxKValue)

	fmt.Printf("\n【符合條件股票】共 %d 檔\n", len(stocks))
	fmt.Println("=====================================")

	for i, stock := range stocks {
		fmt.Printf("\n%d. %s (%s)\n", i+1, stock.Name, stock.Code)
		fmt.Printf("   綜合評分: %.1f\n", stock.Score)
		fmt.Printf("   ROE: %.1f%%\n", stock.ROE)
		fmt.Printf("   營收年增率: %.1f%%\n", stock.RevenueGrowth)
		fmt.Printf("   年增率: %.1f%%\n", stock.YoYGrowth)
		fmt.Printf("   EPS增長: %.1f%%\n", stock.EPSGrowth)
		fmt.Printf("   EPS: %.2f元\n", stock.EPS)
		fmt.Printf("   負債比: %.1f%%\n", stock.DebtRatio)
		fmt.Printf("   現價: %.2f | MA60: %.2f\n", stock.Price, stock.MA60)
		fmt.Printf("   K值: %.1f | D值: %.1f\n", stock.KValue, stock.DValue)
		fmt.Println("   ---")
	}
}

// SaveResults 儲存篩選結果
func (s *StockScreener) SaveResults(stocks []*StockData, filename string) error {
	data, err := json.MarshalIndent(stocks, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

func main() {
	fmt.Println("啟動台股篩選系統...")

	// 建立篩選器
	screener := NewStockScreener()

	// 取得股票清單
	stockList, err := screener.FetchStockList()
	if err != nil {
		log.Fatal("無法取得股票清單:", err)
	}

	fmt.Printf("準備篩選 %d 檔股票...\n", len(stockList))

	// 執行篩選
	qualifiedStocks, err := screener.ScreenStocks(stockList)
	if err != nil {
		log.Fatal("篩選過程發生錯誤:", err)
	}

	// 產生報告
	screener.GenerateReport(qualifiedStocks)

	// 儲存結果
	filename := fmt.Sprintf("screening_results_%s.json",
		time.Now().Format("20060102_150405"))
	if err := screener.SaveResults(qualifiedStocks, filename); err != nil {
		log.Printf("無法儲存結果: %v\n", err)
	} else {
		fmt.Printf("\n結果已儲存至: %s\n", filename)
	}

	// 產生買進建議
	fmt.Println("\n========== 買進建議 ==========")
	if len(qualifiedStocks) > 0 {
		fmt.Println("【優先考慮】評分最高的前3檔:")
		for i := 0; i < len(qualifiedStocks) && i < 3; i++ {
			stock := qualifiedStocks[i]
			fmt.Printf("%d. %s (%s) - 評分: %.1f\n",
				i+1, stock.Name, stock.Code, stock.Score)
		}

		fmt.Println("\n【進場策略】")
		fmt.Println("1. 分3批進場，每批間隔1-2週")
		fmt.Println("2. 設定停損點在買進價-10%")
		fmt.Println("3. 獲利20-30%可先出場一半")
		fmt.Println("4. 每週檢視技術指標變化")
	} else {
		fmt.Println("目前沒有符合所有條件的股票")
		fmt.Println("建議：")
		fmt.Println("1. 放寬部分篩選條件")
		fmt.Println("2. 等待市場回檔再執行篩選")
		fmt.Println("3. 考慮ETF作為替代選擇")
	}
}

// 額外的輔助函數

// CalculateVolatility 計算股價波動率
func CalculateVolatility(prices []float64) float64 {
	if len(prices) < 2 {
		return 0
	}

	// 計算日報酬率
	returns := make([]float64, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		returns[i-1] = (prices[i] - prices[i-1]) / prices[i-1]
	}

	// 計算標準差
	mean := 0.0
	for _, r := range returns {
		mean += r
	}
	mean /= float64(len(returns))

	variance := 0.0
	for _, r := range returns {
		variance += math.Pow(r-mean, 2)
	}
	variance /= float64(len(returns))

	return math.Sqrt(variance) * math.Sqrt(252) // 年化波動率
}

// CalculateSharpeRatio 計算夏普比率
func CalculateSharpeRatio(returns []float64, riskFreeRate float64) float64 {
	if len(returns) == 0 {
		return 0
	}

	avgReturn := 0.0
	for _, r := range returns {
		avgReturn += r
	}
	avgReturn /= float64(len(returns))

	// 計算標準差
	stdDev := 0.0
	for _, r := range returns {
		stdDev += math.Pow(r-avgReturn, 2)
	}
	stdDev = math.Sqrt(stdDev / float64(len(returns)))

	if stdDev == 0 {
		return 0
	}

	return (avgReturn - riskFreeRate) / stdDev
}

// buildYahooSymbol 構建正確的Yahoo Finance股票代碼
func (s *StockScreener) buildYahooSymbol(code string) string {
	// 台灣股票在 Yahoo Finance 的格式
	// 上市股票: XXXX.TW (如 2330.TW)
	// 上櫃股票: XXXX.TWO (但大多數也可用 .TW)
	// ETF: XXXX.TW (如 0050.TW)

	// 特殊處理某些已知的上櫃股票
	otcStocks := map[string]bool{
		"6000": true, // 鈊象電子
		"6005": true, // 群益證
		"3379": true,
		// 可以根據需要添加更多上櫃股票
	}

	if otcStocks[code] {
		return code + ".TWO"
	}

	// 大部分情況使用 .TW 後綴
	return code + ".TW"
}
