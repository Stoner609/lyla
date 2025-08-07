# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Taiwan Stock Market Screening System (台股篩選系統) written in Go. It fetches financial and technical data from Taiwan Stock Exchange APIs and Yahoo Finance to screen stocks based on predefined fundamental and technical criteria.

## Development Commands

### Build and Run
```bash
# Build the application
go build -o stock main.go

# Run directly
go run main.go

# Build and run
go build -o stock main.go && ./stock

# Clean build artifacts
go clean
```

### Testing and Quality
```bash
# Run tests (if any are added)
go test ./...

# Format code
go fmt ./...

# Vet code for issues
go vet ./...

# Download dependencies
go mod download

# Tidy dependencies
go mod tidy
```

## Architecture Overview

The application is structured as a single-file Go program with the following key components:

### Core Data Structures
- **StockData**: Represents a stock with financial metrics (ROE, revenue growth, debt ratio) and technical indicators (MA60, KD values)
- **ScreeningCriteria**: Defines filtering conditions for stock screening
- **StockScreener**: Main service that orchestrates data fetching and screening logic

### Key Components

1. **Data Fetching Layer**
   - `FetchFinancialData()`: Retrieves fundamental data from Taiwan Stock Exchange APIs
   - `FetchTechnicalData()`: Gets technical indicators from Yahoo Finance API
   - `FetchStockList()`: Obtains list of stocks to analyze

2. **Analysis Engine**
   - `calculateTechnicalIndicators()`: Computes MA60, KD values from price data
   - `calculateRSV()`: Calculates Relative Strength Value for KD indicator
   - `meetsScreeningCriteria()`: Filters stocks based on predefined criteria

3. **Scoring System**
   - `calculateScore()`: Assigns composite scores (0-100) based on fundamental (60%) and technical (40%) factors
   - Higher scores indicate better investment prospects

4. **Reporting**
   - `GenerateReport()`: Creates formatted console output with screening results
   - `SaveResults()`: Exports results to JSON file with timestamp

### Default Screening Criteria
- ROE > 15%
- Revenue growth > 0%
- Debt ratio < 40%
- Minimum 5 years of dividend payments
- Stock price above 60-day moving average
- KD values between 50-80 (indicating potential buy zone)

### API Dependencies
- Taiwan Stock Exchange (TWSE) API for fundamental data
- Yahoo Finance API for technical data and price history
- Rate limiting: 1 second delay between requests to avoid overwhelming APIs

### Data Flow
1. Fetch predefined stock list (currently hardcoded with popular Taiwan stocks)
2. For each stock: retrieve financial + technical data
3. Apply screening criteria filters
4. Calculate composite scores for qualifying stocks
5. Sort by score and generate report
6. Save results to timestamped JSON file

## Key Integration Points

When modifying this system, be aware of:

- **API Rate Limits**: Both TWSE and Yahoo Finance APIs have rate limits - maintain the 1-second delay
- **Data Parsing**: Financial data parsing is simplified and may need enhancement for production use
- **Error Handling**: Network failures are logged but don't stop the screening process
- **Stock Universe**: Currently uses a hardcoded list of 20 popular Taiwan stocks - consider expanding or making configurable