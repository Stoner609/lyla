# 台股篩選系統 Taiwan Stock Screening System

A comprehensive stock screening system for Taiwan Stock Exchange (TWSE) that analyzes stocks based on financial fundamentals and technical indicators to identify potential investment opportunities.

## 概述 Overview

本系統結合基本面與技術面分析，從台灣股市中篩選出符合條件的優質股票。系統會自動從台灣證券交易所和Yahoo Finance獲取即時資料，並根據預設的投資策略進行篩選和評分。

This system combines fundamental and technical analysis to screen quality stocks from the Taiwan stock market. It automatically fetches real-time data from Taiwan Stock Exchange and Yahoo Finance APIs, then filters and scores stocks based on predefined investment strategies.

## 主要功能 Key Features

### 基本面分析 Fundamental Analysis
- **ROE (股東權益報酬率)**: 評估企業獲利能力，採用精確的淨利/平均權益計算
- **營收成長率**: 分析企業成長動能，支援同季比較  
- **年增率 (YoY Growth)**: 年對年成長率分析
- **EPS增長率**: 每股盈餘增長幅度評估
- **負債比**: 評估財務結構健全度
- **配息穩定性**: 檢查穩定配息記錄

### 技術面分析 Technical Analysis
- **60日移動平均線 (MA60)**: 判斷中期趨勢
- **KD指標**: 判斷買賣時機點
- **價格動能**: 確認股價位置相對強弱

### 評分系統 Scoring System
- **綜合評分**: 基本面佔70%，技術面佔30%
- **多階段篩選**: 基本財務健康度 → 投資品質評估 → 技術面時機
- **動態排序**: 自動按評分高低排列結果
- **風險評估**: 包含波動率和夏普比率計算

### 報告功能 Reporting Features
- **即時篩選報告**: 詳細的股票分析結果
- **JSON數據導出**: 可供進一步分析使用
- **買進策略建議**: 提供具體的投資建議

## 預設篩選條件 Default Screening Criteria

### 第一階段：基本財務健康度 (必須條件)
| 條件 Criteria | 數值 Value | 說明 Description |
|---------------|-----------|-----------------|
| ROE | > 0% | 排除虧損企業 |
| 負債比 Debt Ratio | < 80% | 避免過度負債 |
| 營收成長率 Revenue Growth | > -20% | 排除大幅衰退 |
| 年增率 YoY Growth | > -30% | 排除嚴重衰退 |
| EPS增長率 EPS Growth | > -50% | 排除獲利大幅下滑 |

### 第二階段：投資品質評估 (優選條件)
| 條件 Criteria | 數值 Value | 說明 Description |
|---------------|-----------|-----------------|
| ROE | ≥ 8% | 合理獲利能力 |
| 營收成長率 Revenue Growth | ≥ -5% | 允許小幅調整 |
| 年增率 YoY Growth | ≥ 10% | 成長動能要求 |
| EPS增長率 EPS Growth | ≥ 100% | 三位數增長期待 |
| EPS | ≥ 1.0元 | 基本獲利水準 |
| 負債比 Debt Ratio | ≤ 60% | 財務結構穩健 |
| 配息年數 Dividend Years | ≥ 2年 | 基本配息記錄 |

### 第三階段：技術面時機 (參考條件)
| 條件 Criteria | 數值 Value | 說明 Description |
|---------------|-----------|-----------------|
| MA60位置 | 可選擇性要求 | 中期趨勢參考 |
| KD值 KD Values | 30-85 | 擴大觀察區間 |

## 系統架構 System Architecture

```
├── main.go                 # 主程式檔案
├── README.md              # 專案說明文件  
├── CLAUDE.md              # 開發指導文件
└── go.mod                 # Go模組依賴
```

### 核心資料結構 Core Data Structures

- **StockData**: 股票完整資訊結構
- **ScreeningCriteria**: 篩選條件定義
- **StockScreener**: 主要篩選引擎

### API數據來源 API Data Sources

- **FinMind API**: 主要財務數據來源 (損益表、資產負債表)
- **Taiwan Stock Exchange (TWSE)**: 補充財務資料
- **Yahoo Finance**: 技術面與價格資料

## 安裝與使用 Installation & Usage

### 系統需求 Prerequisites
- Go 1.19 或以上版本
- 網路連線 (用於API請求)

### 安裝步驟 Installation Steps

1. 複製專案
```bash
git clone <repository-url>
cd stock
```

2. 下載相依套件
```bash
go mod download
```

### 執行程式 Running the Application

#### 直接執行 Direct Run
```bash
go run main.go
```

#### 編譯後執行 Build and Run
```bash
# 編譯
go build -o stock main.go

# 執行
./stock
```

### 開發指令 Development Commands

```bash
# 格式化程式碼
go fmt ./...

# 檢查程式碼品質
go vet ./...

# 執行測試
go test ./...

# 整理依賴
go mod tidy

# 清除編譯檔案
go clean
```

## 輸出結果 Output Results

### 控制台報告 Console Report
程式會即時顯示：
- 篩選條件摘要
- 符合條件的股票清單
- 詳細的股票分析資料
- 投資建議與策略

### JSON檔案 JSON Export
系統會自動生成時間戳記的JSON檔案：
```
screening_results_20240107_143052.json
```

包含所有篩選結果的完整資料，可用於：
- 歷史資料比較分析
- 第三方工具整合
- 進一步的量化分析

## 分析股票清單 Stock Universe

目前分析以下20檔熱門台股：

| 股票代碼 | 股票名稱 | 類別 |
|---------|---------|------|
| 2330 | 台積電 | 科技 |
| 2454 | 聯發科 | 科技 |
| 2308 | 台達電 | 電子 |
| 2886 | 兆豐金 | 金融 |
| 2884 | 玉山金 | 金融 |
| 0050 | 元大台灣50 | ETF |
| 0056 | 元大高股息 | ETF |
| ... | ... | ... |

## 投資策略建議 Investment Strategy

### 進場策略 Entry Strategy
1. **分批進場**: 建議分3批進場，每批間隔1-2週
2. **技術確認**: 確保股價站穩60日均線之上
3. **KD指標**: 在50-80區間為較佳進場時機

### 風險控制 Risk Management
1. **停損設定**: 建議設在買進價-10%
2. **獲利了結**: 獲利20-30%可先出場一半
3. **定期檢視**: 每週檢視技術指標變化

## 系統限制與注意事項 Limitations & Notes

### API限制 API Limitations
- **請求頻率**: 系統設有1秒間隔避免過度請求
- **資料延遲**: 某些資料可能有15-20分鐘延遲
- **假日限制**: 週末及國定假日無法取得即時資料

### 資料準確性 Data Accuracy
- 基本面資料經過簡化處理，實際投資前請查證
- 技術指標計算基於歷史價格，不保證未來表現
- 系統僅供參考，投資決策請自行承擔風險

## 財務指標計算方法 Financial Calculation Methods

### ROE (股東權益報酬率) 精確計算法
本系統採用精確的ROE計算方法：
1. **分子**: 從FinMind API獲取最新季度的稅後淨利 (`IncomeAfterTaxes`)
2. **分母**: 取當季末和前季末權益總額的平均值
3. **公式**: ROE = 本期淨利 / 平均股東權益 × 100%

**範例** (以廣宇2328為例):
```
本期淨利: 269,357,000 元 (2025Q1)
平均權益: (16,503,255,000 + 16,485,867,000) / 2 = 16,494,561,000 元
ROE = 269,357,000 / 16,494,561,000 × 100% = 1.63%
```

### 年增率 (YoY Growth) 同季比較
- 採用相同季度的年度對比，避免季節性影響
- 支援營收、EPS等關鍵指標的年增率計算

### 資料來源整合
- **FinMind API**: 提供準確的損益表和資產負債表數據
- **自動容錯**: API失敗時使用TWSE數據作為備用
- **三層備案**: FinMind → TWSE → 行業估算

## 技術指標說明 Technical Indicators

### 移動平均線 (MA60)
- 計算過去60個交易日的平均價格
- 股價在MA60之上視為多頭趨勢
- 可作為支撐壓力參考點

### KD指標
- K值：快速指標，反應短期買賣力道
- D值：慢速指標，K值的移動平均
- 30-85區間：擴大觀察範圍，涵蓋更多投資機會
- 50-80區間：相對安全的買進區域

## 客製化設定 Customization

### 修改篩選條件
在 `main.go` 中的 `NewStockScreener()` 函數修改：

```go
criteria: ScreeningCriteria{
    MinROE:           8.0,   // 最低ROE要求
    MinRevenueGrowth: -5.0,  // 最低營收成長率 (允許小幅衰退)
    MaxDebtRatio:     60.0,  // 最高負債比
    MinDividendYears: 2,     // 最少配息年數
    MinYoYGrowth:     10.0,  // 年增率至少10%
    MinEPSGrowth:     100.0, // EPS增長至少100% (三位數增長)
    MinEPS:           1.0,   // 最小EPS要求1元
    RequireMA60Above: false, // 是否須站上MA60
    MinKValue:        30.0,  // KD值範圍
    MaxKValue:        85.0,
    MinDValue:        30.0,
    MaxDValue:        85.0,
}
```

### 擴充股票清單
在 `FetchStockList()` 函數中新增股票代碼：

```go
stockList := []string{
    "2330", // 台積電
    "XXXX", // 新增股票代碼
    // ...
}
```

## 故障排除 Troubleshooting

### 常見問題 Common Issues

1. **網路連線問題**
```
Error: Get "https://...": dial tcp: i/o timeout
```
解決方案：檢查網路連線，稍後重試

2. **API資料格式錯誤**
```
JSON 解析錯誤: invalid character...
```
解決方案：API可能暫時無法使用，或資料格式變更

3. **股票代碼錯誤**
```
Yahoo Finance API 錯誤: Not Found
```
解決方案：檢查股票代碼是否正確，或該股票已下市

## 最近更新 Recent Updates

### v2.0.0 - 2025年8月 (最新)
- ✅ **精確ROE計算**: 整合FinMind API，採用標準財務公式
- ✅ **多階段篩選**: 實施三階段漸進式篩選機制
- ✅ **新增YoY/EPS成長指標**: 增強基本面分析維度
- ✅ **評分權重調整**: 基本面70%，技術面30%
- ✅ **智慧容錯機制**: FinMind → TWSE → 行業估算三層備案

### v1.0.0 - 初始版本
- ✅ 基礎股票篩選功能
- ✅ TWSE和Yahoo Finance API整合
- ✅ 基本技術指標計算

## 未來發展 Future Development

### 計劃功能 Planned Features
- [ ] 更多技術指標支援 (RSI, MACD, 布林通道)
- [ ] 季報/年報深度分析
- [ ] 網頁介面開發
- [ ] 郵件通知系統
- [ ] 歷史回測功能
- [ ] 投組建構建議

### 效能優化 Performance Optimization
- [ ] 資料庫快取機制
- [ ] 平行處理改善
- [ ] 記憶體使用優化
- [ ] API請求頻率最佳化

## 授權資訊 License

本專案僅供學習研究用途，不構成投資建議。使用本系統進行投資決策的風險由使用者自行承擔。

This project is for educational and research purposes only. Investment decisions based on this system are at your own risk.

## 貢獻指南 Contributing

歡迎提交Issue和Pull Request來改善本專案。請確保：

1. 遵循Go語言編碼規範
2. 提供適當的測試
3. 更新相關文檔

## 聯絡資訊 Contact

如有問題或建議，請透過GitHub Issue聯繫。

---

**免責聲明**: 本系統提供的資訊僅供參考，不構成投資建議。投資有風險，請謹慎評估自身財務狀況後進行投資決策。

**Disclaimer**: The information provided by this system is for reference only and does not constitute investment advice. Investment involves risks; please carefully evaluate your financial situation before making investment decisions.