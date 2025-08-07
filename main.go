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
	RequireMA60Above bool
	MinKValue        float64
	MaxKValue        float64
	MinDValue        float64
	MaxDValue        float64
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
			MinROE:           15.0,
			MinRevenueGrowth: 0.0,
			MaxDebtRatio:     40.0,
			MinDividendYears: 5,
			RequireMA60Above: true,
			MinKValue:        50.0,
			MaxKValue:        80.0,
			MinDValue:        50.0,
			MaxDValue:        80.0,
		},
	}
}

// FetchFinancialData 從公開資訊觀測站取得財務資料
func (s *StockScreener) FetchFinancialData(stockCode string) (*StockData, error) {
	stock := &StockData{
		Code: stockCode,
	}

	// 取得基本面資料 (使用證交所API)
	fundamentalURL := fmt.Sprintf("https://www.twse.com.tw/exchangeReport/BWIBBU_d?response=json&date=%s&stockNo=%s",
		time.Now().Format("20060102"), stockCode)

	resp, err := s.client.Get(fundamentalURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 解析JSON回應
	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
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

	return stock, nil
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
		"2330", // 台積電
		"2454", // 聯發科
		"2308", // 台達電
		"2886", // 兆豐金
		"2884", // 玉山金
		"2382", // 廣達
		"3231", // 緯創
		"2376", // 技嘉
		"2449", // 京元電
		"1216", // 統一
		"2412", // 中華電
		"0050", // 元大台灣50
		"0056", // 元大高股息
		"2603", // 長榮
		"2609", // 陽明
		"2881", // 富邦金
		"2882", // 國泰金
		"2892", // 第一金
		"3008", // 大立光
		"2317", // 鴻海
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

// meetsScreeningCriteria 檢查是否符合篩選條件
func (s *StockScreener) meetsScreeningCriteria(stock *StockData) bool {
	// 基本面條件
	if stock.ROE < s.criteria.MinROE {
		return false
	}
	if stock.RevenueGrowth < s.criteria.MinRevenueGrowth {
		return false
	}
	if stock.DebtRatio > s.criteria.MaxDebtRatio {
		return false
	}
	if stock.DividendYears < s.criteria.MinDividendYears {
		return false
	}

	// 技術面條件
	if s.criteria.RequireMA60Above && stock.Price < stock.MA60 {
		return false
	}
	if stock.KValue < s.criteria.MinKValue || stock.KValue > s.criteria.MaxKValue {
		return false
	}
	if stock.DValue < s.criteria.MinDValue || stock.DValue > s.criteria.MaxDValue {
		return false
	}

	return true
}

// calculateScore 計算綜合評分
func (s *StockScreener) calculateScore(stock *StockData) {
	score := 0.0

	// 基本面評分 (60%)
	score += math.Min(stock.ROE/30.0, 1.0) * 20                    // ROE評分
	score += math.Min(stock.RevenueGrowth/20.0, 1.0) * 15          // 營收成長評分
	score += (1.0 - stock.DebtRatio/100.0) * 15                    // 負債比評分
	score += math.Min(float64(stock.DividendYears)/10.0, 1.0) * 10 // 配息穩定性

	// 技術面評分 (40%)
	if stock.Price > stock.MA60 {
		score += 20 // 站上季線
	}

	// KD值在黃金交叉區間
	if stock.KValue >= 50 && stock.KValue <= 80 {
		score += 10
	}
	if stock.DValue >= 50 && stock.DValue <= 80 {
		score += 10
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
		// 可以根據需要添加更多上櫃股票
	}
	
	if otcStocks[code] {
		return code + ".TWO"
	}
	
	// 大部分情況使用 .TW 後綴
	return code + ".TW"
}
