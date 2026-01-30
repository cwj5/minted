package hledger

import (
	"encoding/json"
	"log"
	"math"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/cwj5/minted/internal/config"
)

// Account represents an hledger account
type Account struct {
	Name     string  `json:"aname"`
	Balance  float64 `json:"aebalance"`
	Currency string  `json:"currency"`
}

// Transaction represents a transaction
type Transaction struct {
	Date        string    `json:"tdate"`
	Description string    `json:"tdescription"`
	Postings    []Posting `json:"tpostings"`
}

// Posting represents a posting within a transaction
type Posting struct {
	Account string   `json:"paccount"`
	Amount  []Amount `json:"pamount"`
	Comment string   `json:"pcomment"`
}

// Amount represents a monetary amount with commodity
type Amount struct {
	Commodity string   `json:"acommodity"`
	Quantity  Quantity `json:"aquantity"`
}

// Quantity represents the numeric quantity
type Quantity struct {
	DecimalMantissa int64 `json:"decimalMantissa"`
	DecimalPlaces   int   `json:"decimalPlaces"`
}

// BudgetItem represents budget information for a spending category
type BudgetItem struct {
	Category      string  `json:"category"`
	Average       float64 `json:"average"`
	CurrentMonth  float64 `json:"currentMonth"`
	Variance      float64 `json:"variance"`
	PercentBudget float64 `json:"percentBudget"`
}

// MonthBudget represents spend for a category in a given month
type MonthBudget struct {
	Month           string  `json:"month"`
	Year            string  `json:"year"`
	Amount          float64 `json:"amount"`
	PercentOfBudget float64 `json:"percentOfBudget"`
	OverBudget      bool    `json:"overBudget"`
}

// BudgetHistoryItem holds historical spending against the average
type BudgetHistoryItem struct {
	Category                 string        `json:"category"`
	Average                  float64       `json:"average"`
	AverageExcludingExtremes float64       `json:"averageExcludingExtremes"`
	Months                   []MonthBudget `json:"months"`
}

// MonthlyMetrics represents financial metrics for a month
type MonthlyMetrics struct {
	Month       string  `json:"month"`
	Income      float64 `json:"income"`
	Expenses    float64 `json:"expenses"`
	NetWorth    float64 `json:"netWorth"`
	SavingsRate float64 `json:"savingsRate"`
}

// CategorySpending represents spending for a category in a month
type CategorySpending struct {
	Month    string  `json:"month"`
	Category string  `json:"category"`
	Amount   float64 `json:"amount"`
}

// NetWorthPoint represents net worth at a specific point in time
type NetWorthPoint struct {
	Date     string  `json:"date"`
	NetWorth float64 `json:"netWorth"`
}

// CategoryTrendData represents spending trend for a single category
type CategoryTrendData struct {
	Category string            `json:"category"`
	Data     []MonthAmountPair `json:"data"`
}

// MonthAmountPair represents a month and amount
type MonthAmountPair struct {
	Month  string  `json:"month"`
	Amount float64 `json:"amount"`
}

// YearOverYearData represents same-month comparison across years
type YearOverYearData struct {
	Month string             `json:"month"` // e.g., "01" for January
	Years map[string]float64 `json:"years"` // year -> spending amount, e.g., "2024" -> 500.00
}

// SubcategoryBreakdown represents spending breakdown by subcategories
type SubcategoryBreakdown struct {
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
}

// CategoryDetailData represents detailed view data for a category
type CategoryDetailData struct {
	Category      string                 `json:"category"`
	Transactions  []Transaction          `json:"transactions"`
	BudgetHistory []BudgetHistoryItem    `json:"budgetHistory"`
	Breakdown     []SubcategoryBreakdown `json:"breakdown"`
}

// TierDetailData represents detailed view data for a tier
type TierDetailData struct {
	Tier          string                 `json:"tier"`
	Transactions  []Transaction          `json:"transactions"`
	BudgetHistory []BudgetHistoryItem    `json:"budgetHistory"`
	Breakdown     []SubcategoryBreakdown `json:"breakdown"`
}

// AccountDetailData represents detailed view data for an account
type AccountDetailData struct {
	Account        string                `json:"account"`
	Transactions   []Transaction         `json:"transactions"`
	BalanceHistory []BalanceHistoryPoint `json:"balanceHistory"`
}

// BalanceHistoryPoint represents account balance at a point in time
type BalanceHistoryPoint struct {
	Date    string  `json:"date"`
	Balance float64 `json:"balance"`
}

// Parser handles hledger journal parsing
type Parser struct {
	journalFile string
	settings    *config.Settings
}

// NewParser creates a new hledger parser
func NewParser(journalFile string, settings *config.Settings) *Parser {
	return &Parser{
		journalFile: journalFile,
		settings:    settings,
	}
}

// UpdateSettings updates the parser's settings (used when settings change at runtime)
func (p *Parser) UpdateSettings(settings *config.Settings) {
	p.settings = settings
}

// buildDateArgs constructs hledger command line args for date filtering
func (p *Parser) buildDateArgs(startDate, endDate string) []string {
	if startDate == "" || endDate == "" {
		return []string{}
	}
	// Use -e (end date is exclusive in hledger)
	return []string{"-b", startDate, "-e", endDate}
}

// GetAccounts retrieves Assets and Liabilities accounts from hledger with their balances
func (p *Parser) GetAccounts() ([]Account, error) {
	cmd := exec.Command("hledger", "-f", p.journalFile, "balance", "--empty", "-O", "json")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Error running hledger balance: file=%s, error=%v", p.journalFile, err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			log.Printf("stderr: %s", string(exitErr.Stderr))
		}
		return nil, err
	}

	// Balance JSON structure: [[account_entry1, account_entry2, ...], [total]]
	var balanceData [][]interface{}
	err = json.Unmarshal(output, &balanceData)
	if err != nil {
		log.Printf("Error parsing JSON: %v", err)
		return nil, err
	}

	var accounts []Account

	// The first element contains all the account entries
	if len(balanceData) > 0 {
		accountsList := balanceData[0]

		// Process each account
		for _, item := range accountsList {
			if itemArr, ok := item.([]interface{}); ok && len(itemArr) >= 4 {
				// First element is the account name
				name, ok := itemArr[0].(string)
				if !ok || name == "" {
					continue
				}

				// Only include Assets and Liabilities accounts in the main accounts section
				if !strings.HasPrefix(name, "assets:") && !strings.HasPrefix(name, "liabilities:") {
					continue
				}

				var balance float64 = 0

				// Fourth element is the array of amounts
				if amounts, ok := itemArr[3].([]interface{}); ok && len(amounts) > 0 {
					if amountObj, ok := amounts[0].(map[string]interface{}); ok {
						if qty, ok := amountObj["aquantity"].(map[string]interface{}); ok {
							if mantissa, ok := qty["decimalMantissa"].(float64); ok {
								if places, ok := qty["decimalPlaces"].(float64); ok {
									// Convert decimalMantissa and decimalPlaces to actual value
									divisor := 1.0
									for i := 0; i < int(places); i++ {
										divisor *= 10
									}
									balance = mantissa / divisor
								}
							}
						}
					}
				}

				accounts = append(accounts, Account{
					Name:     name,
					Balance:  balance,
					Currency: "$",
				})
			}
		}
	}

	return accounts, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetTransactions retrieves recent transactions
func (p *Parser) GetTransactions() ([]Transaction, error) {
	cmd := exec.Command("hledger", "-f", p.journalFile, "print", "-O", "json")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Error running hledger print: %v", err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			log.Printf("stderr: %s", string(exitErr.Stderr))
		}
		return nil, err
	}

	var transactions []Transaction
	err = json.Unmarshal(output, &transactions)
	if err != nil {
		log.Printf("Error parsing JSON: %v", err)
		return nil, err
	}

	return transactions, nil
}

// GetAccountBalance retrieves the balance of a specific account
func (p *Parser) GetAccountBalance(account string) (float64, error) {
	cmd := exec.Command("hledger", "-f", p.journalFile, "balance", account, "-O", "json")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Error running hledger: %v", err)
		return 0, err
	}

	var accounts []Account
	err = json.Unmarshal(output, &accounts)
	if err != nil {
		log.Printf("Error parsing JSON: %v", err)
		return 0, err
	}

	if len(accounts) > 0 {
		// Convert balance to float
		// For now, return 0 as we need better logic to handle the complex balance format
		return 0, nil
	}

	return 0, nil
}

// convertAmount converts hledger quantity to float64
func convertAmount(quantity Quantity) float64 {
	divisor := 1.0
	for i := 0; i < quantity.DecimalPlaces; i++ {
		divisor *= 10
	}
	return float64(quantity.DecimalMantissa) / divisor
}

// getYearMonth extracts YYYY-MM from date string YYYY-MM-DD
func getYearMonth(dateStr string) string {
	if len(dateStr) >= 7 {
		return dateStr[:7]
	}
	return dateStr
}

// getCurrentYearMonth returns the current month in YYYY-MM format
func getCurrentYearMonth() string {
	return time.Now().Format("2006-01")
}

// GetMonthlySpending aggregates expenses by category and month
func (p *Parser) GetMonthlySpending() (map[string]map[string]float64, error) {
	transactions, err := p.GetTransactions()
	if err != nil {
		return nil, err
	}

	// Map of month -> category -> total amount
	monthlyByCategory := make(map[string]map[string]float64)

	for _, tx := range transactions {
		month := getYearMonth(tx.Date)

		for _, posting := range tx.Postings {
			// Only include Expenses accounts
			if !strings.HasPrefix(posting.Account, "expenses:") {
				continue
			}

			// Extract category (second part of account name)
			parts := strings.Split(posting.Account, ":")
			var category string
			if len(parts) >= 2 {
				category = parts[1]
			} else {
				category = posting.Account
			}

			// Get amount (use absolute value for expenses)
			var amount float64
			if len(posting.Amount) > 0 {
				amount = convertAmount(posting.Amount[0].Quantity)
			}

			// Store positive value for expenses
			if amount < 0 {
				amount = -amount
			}

			// Initialize month map if needed
			if monthlyByCategory[month] == nil {
				monthlyByCategory[month] = make(map[string]float64)
			}

			monthlyByCategory[month][category] += amount
		}
	}

	return monthlyByCategory, nil
}

// removeOutliers removes the highest and lowest values from a slice using IQR method
func removeOutliers(values []float64) []float64 {
	if len(values) <= 2 {
		return values
	}

	sort.Float64s(values)

	// Calculate Q1 and Q3
	q1Index := len(values) / 4
	q3Index := (len(values) * 3) / 4

	if q1Index == q3Index {
		// Not enough data points
		return values
	}

	q1 := values[q1Index]
	q3 := values[q3Index]
	iqr := q3 - q1

	// Lower and upper bounds (Q1 - 1.5*IQR, Q3 + 1.5*IQR)
	lowerBound := q1 - 1.5*iqr
	upperBound := q3 + 1.5*iqr

	var filtered []float64
	for _, v := range values {
		if v >= lowerBound && v <= upperBound {
			filtered = append(filtered, v)
		}
	}

	return filtered
}

// GetBudgetHistory returns per-category spend by month with percent vs average
func (p *Parser) GetBudgetHistory() ([]BudgetHistoryItem, error) {
	monthlySpending, err := p.GetMonthlySpending()
	if err != nil {
		return nil, err
	}

	currentMonth := getCurrentYearMonth()

	// Collect all months and extract unique years
	var allMonths []string
	for month := range monthlySpending {
		allMonths = append(allMonths, month)
	}
	sort.Strings(allMonths)

	// Build category history excluding current month for averages
	categoryHistory := make(map[string][]float64)
	for month, categories := range monthlySpending {
		if month == currentMonth {
			continue
		}
		for category, amount := range categories {
			categoryHistory[category] = append(categoryHistory[category], amount)
		}
	}

	var history []BudgetHistoryItem

	for category, amounts := range categoryHistory {
		if len(amounts) < 2 {
			// Need at least two months to establish a reasonable average
			continue
		}

		var sum float64
		for _, v := range amounts {
			sum += v
		}
		avg := sum / float64(len(amounts))

		// Calculate average excluding extremes (values > 2x average)
		var filteredAmounts []float64
		for _, v := range amounts {
			if v <= avg*2 {
				filteredAmounts = append(filteredAmounts, v)
			}
		}
		avgExcludingExtremes := avg
		if len(filteredAmounts) > 0 {
			var filteredSum float64
			for _, v := range filteredAmounts {
				filteredSum += v
			}
			avgExcludingExtremes = filteredSum / float64(len(filteredAmounts))
		}

		var monthData []MonthBudget
		for _, month := range allMonths {
			var amount float64
			if categories, ok := monthlySpending[month]; ok {
				amount = categories[category]
			}

			percent := 0.0
			if avg > 0 {
				percent = (amount / avg) * 100
			}

			// Extract year from month (format: YYYY-MM)
			year := ""
			if len(month) >= 4 {
				year = month[:4]
			}

			monthData = append(monthData, MonthBudget{
				Month:           month,
				Year:            year,
				Amount:          math.Round(amount*100) / 100,
				PercentOfBudget: math.Round(percent*100) / 100,
				OverBudget:      amount > avg,
			})
		}

		history = append(history, BudgetHistoryItem{
			Category:                 category,
			Average:                  math.Round(avg*100) / 100,
			AverageExcludingExtremes: math.Round(avgExcludingExtremes*100) / 100,
			Months:                   monthData,
		})
	}

	sort.Slice(history, func(i, j int) bool {
		return history[i].Category < history[j].Category
	})

	return history, nil
}

// GetBudgetData calculates budget targets based on historical spending averages
func (p *Parser) GetBudgetData() ([]BudgetItem, error) {
	monthlySpending, err := p.GetMonthlySpending()
	if err != nil {
		return nil, err
	}

	// Map of category -> list of monthly amounts
	categoryHistory := make(map[string][]float64)
	currentMonth := getCurrentYearMonth()

	for month, categories := range monthlySpending {
		// Skip current month from budget calculation
		if month == currentMonth {
			continue
		}

		for category, amount := range categories {
			categoryHistory[category] = append(categoryHistory[category], amount)
		}
	}

	// Get current month spending
	currentMonthSpending := make(map[string]float64)
	if current, exists := monthlySpending[currentMonth]; exists {
		currentMonthSpending = current
	}

	var budgetItems []BudgetItem

	// Calculate averages and variances
	for category, amounts := range categoryHistory {
		// Only include categories with at least 2 months of history
		if len(amounts) < 2 {
			continue
		}

		// Remove outliers
		filtered := removeOutliers(amounts)

		// Calculate average
		var average float64
		for _, v := range filtered {
			average += v
		}
		average /= float64(len(filtered))

		// Get current month spending
		current := currentMonthSpending[category]

		// Calculate variance
		variance := current - average

		// Calculate percent of budget
		percentBudget := 0.0
		if average > 0 {
			percentBudget = (current / average) * 100
		}

		budgetItems = append(budgetItems, BudgetItem{
			Category:      category,
			Average:       math.Round(average*100) / 100, // Round to 2 decimals
			CurrentMonth:  math.Round(current*100) / 100,
			Variance:      math.Round(variance*100) / 100,
			PercentBudget: math.Round(percentBudget*100) / 100,
		})
	}

	// Sort by category name
	sort.Slice(budgetItems, func(i, j int) bool {
		return budgetItems[i].Category < budgetItems[j].Category
	})

	return budgetItems, nil
}

// GetMonthlyMetrics returns income, expenses, and net worth for each month
func (p *Parser) GetMonthlyMetrics() ([]MonthlyMetrics, error) {
	transactions, err := p.GetTransactions()
	if err != nil {
		return nil, err
	}

	// Map of month -> {income, expenses}
	monthlyData := make(map[string]struct {
		income   float64
		expenses float64
	})

	for _, tx := range transactions {
		month := getYearMonth(tx.Date)

		for _, posting := range tx.Postings {
			var amount float64
			if len(posting.Amount) > 0 {
				amount = convertAmount(posting.Amount[0].Quantity)
			}

			// Positive amounts for income (convert negative to positive), negative for expenses
			if strings.HasPrefix(posting.Account, "income:") {
				data := monthlyData[month]
				data.income += -amount // Income is negative in hledger, so negate it
				monthlyData[month] = data
			} else if strings.HasPrefix(posting.Account, "expenses:") {
				data := monthlyData[month]
				data.expenses += amount
				monthlyData[month] = data
			}
		}
	}

	// Get all unique months and sort them
	var months []string
	for m := range monthlyData {
		months = append(months, m)
	}
	sort.Strings(months)

	// Build metrics
	var metrics []MonthlyMetrics
	for _, month := range months {
		data := monthlyData[month]

		// Get net worth for this month
		netWorth := 0.0
		// This is a simplified version - for complete accuracy we'd need to calculate
		// balance at each point in time, which is more complex

		// Calculate savings rate
		savingsRate := 0.0
		if data.income > 0 {
			savingsRate = ((data.income - data.expenses) / data.income) * 100
		}

		metrics = append(metrics, MonthlyMetrics{
			Month:       month,
			Income:      math.Round(data.income*100) / 100,
			Expenses:    math.Round(data.expenses*100) / 100,
			NetWorth:    netWorth,
			SavingsRate: math.Round(savingsRate*100) / 100,
		})
	}

	return metrics, nil
}

// GetIncomeHistory returns income by category for each month, for historical tracking
func (p *Parser) GetIncomeHistory() ([]BudgetHistoryItem, error) {
	transactions, err := p.GetTransactions()
	if err != nil {
		return nil, err
	}

	// Map of month -> {category -> amount}
	monthlyIncome := make(map[string]map[string]float64)

	for _, tx := range transactions {
		month := getYearMonth(tx.Date)

		for _, posting := range tx.Postings {
			if !strings.HasPrefix(posting.Account, "income:") {
				continue
			}

			// Extract income category
			parts := strings.Split(posting.Account, ":")
			var category string
			if len(parts) >= 2 {
				category = parts[1]
			} else {
				category = posting.Account
			}

			var amount float64
			if len(posting.Amount) > 0 {
				amount = convertAmount(posting.Amount[0].Quantity)
			}

			// Income is negative in hledger, so negate it for positive display
			if amount < 0 {
				amount = -amount
			}

			if monthlyIncome[month] == nil {
				monthlyIncome[month] = make(map[string]float64)
			}
			monthlyIncome[month][category] += amount
		}
	}

	// Get all unique months and sort them
	var allMonths []string
	for month := range monthlyIncome {
		allMonths = append(allMonths, month)
	}
	sort.Strings(allMonths)

	currentMonth := getCurrentYearMonth()

	// Build category history excluding current month for averages
	categoryHistory := make(map[string][]float64)
	for month, categories := range monthlyIncome {
		if month == currentMonth {
			continue
		}
		for category, amount := range categories {
			categoryHistory[category] = append(categoryHistory[category], amount)
		}
	}

	var history []BudgetHistoryItem

	for category, amounts := range categoryHistory {
		if len(amounts) < 2 {
			// Need at least two months to establish a reasonable average
			continue
		}

		var sum float64
		for _, v := range amounts {
			sum += v
		}
		avg := sum / float64(len(amounts))

		// Calculate average excluding extremes
		var filteredAmounts []float64
		for _, v := range amounts {
			if v <= avg*2 {
				filteredAmounts = append(filteredAmounts, v)
			}
		}
		avgExcludingExtremes := avg
		if len(filteredAmounts) > 0 {
			var filteredSum float64
			for _, v := range filteredAmounts {
				filteredSum += v
			}
			avgExcludingExtremes = filteredSum / float64(len(filteredAmounts))
		}

		var monthData []MonthBudget
		for _, month := range allMonths {
			var amount float64
			if categories, ok := monthlyIncome[month]; ok {
				amount = categories[category]
			}

			percent := 0.0
			if avg > 0 {
				percent = (amount / avg) * 100
			}

			// Extract year from month (format: YYYY-MM)
			year := ""
			if len(month) >= 4 {
				year = month[:4]
			}

			monthData = append(monthData, MonthBudget{
				Month:           month,
				Year:            year,
				Amount:          math.Round(amount*100) / 100,
				PercentOfBudget: math.Round(percent*100) / 100,
				OverBudget:      false, // Not applicable for income
			})
		}

		history = append(history, BudgetHistoryItem{
			Category:                 category,
			Average:                  math.Round(avg*100) / 100,
			AverageExcludingExtremes: math.Round(avgExcludingExtremes*100) / 100,
			Months:                   monthData,
		})
	}

	sort.Slice(history, func(i, j int) bool {
		return history[i].Category < history[j].Category
	})

	return history, nil
}

// GetCategorySpending returns spending by category for each month
func (p *Parser) GetCategorySpending() ([]CategorySpending, error) {
	transactions, err := p.GetTransactions()
	if err != nil {
		return nil, err
	}

	// Map of month -> category -> amount
	monthlyCategories := make(map[string]map[string]float64)

	for _, tx := range transactions {
		month := getYearMonth(tx.Date)

		for _, posting := range tx.Postings {
			// Only include Expenses accounts
			if !strings.HasPrefix(posting.Account, "expenses:") {
				continue
			}

			// Extract category
			parts := strings.Split(posting.Account, ":")
			var category string
			if len(parts) >= 2 {
				category = parts[1]
			} else {
				category = posting.Account
			}

			var amount float64
			if len(posting.Amount) > 0 {
				amount = convertAmount(posting.Amount[0].Quantity)
			}

			// Store positive value for expenses
			if amount < 0 {
				amount = -amount
			}

			if monthlyCategories[month] == nil {
				monthlyCategories[month] = make(map[string]float64)
			}
			monthlyCategories[month][category] += amount
		}
	}

	// Build result
	var result []CategorySpending
	for month, categories := range monthlyCategories {
		for category, amount := range categories {
			result = append(result, CategorySpending{
				Month:    month,
				Category: category,
				Amount:   math.Round(amount*100) / 100,
			})
		}
	}

	// Sort by month and category
	sort.Slice(result, func(i, j int) bool {
		if result[i].Month != result[j].Month {
			return result[i].Month < result[j].Month
		}
		return result[i].Category < result[j].Category
	})

	return result, nil
}

// GetIncomeBreakdown returns income categories aggregated across all months
func (p *Parser) GetIncomeBreakdown() ([]CategorySpending, error) {
	transactions, err := p.GetTransactions()
	if err != nil {
		return nil, err
	}

	// Map of category -> total amount
	incomeCategories := make(map[string]float64)

	for _, tx := range transactions {
		for _, posting := range tx.Postings {
			// Only include Income accounts
			if !strings.HasPrefix(posting.Account, "income:") {
				continue
			}

			// Extract category
			parts := strings.Split(posting.Account, ":")
			var category string
			if len(parts) >= 2 {
				category = parts[1]
			} else {
				category = posting.Account
			}

			var amount float64
			if len(posting.Amount) > 0 {
				amount = convertAmount(posting.Amount[0].Quantity)
			}

			// Income amounts are typically negative in hledger, make them positive
			if amount < 0 {
				amount = -amount
			}

			incomeCategories[category] += amount
		}
	}

	// Build result
	var result []CategorySpending
	for category, amount := range incomeCategories {
		result = append(result, CategorySpending{
			Month:    "", // Not monthly, so leave empty
			Category: category,
			Amount:   math.Round(amount*100) / 100,
		})
	}

	// Sort by amount (descending) to show largest income source first
	sort.Slice(result, func(i, j int) bool {
		return result[i].Amount > result[j].Amount
	})

	return result, nil
}

// GetNetWorthOverTime calculates net worth for each day with transactions
func (p *Parser) GetNetWorthOverTime() ([]NetWorthPoint, error) {
	transactions, err := p.GetTransactions()
	if err != nil {
		return nil, err
	}

	// Track cumulative balance by account
	accountBalances := make(map[string]float64)
	dailyNetWorth := make(map[string]float64)

	// Get all transactions sorted by date
	sort.Slice(transactions, func(i, j int) bool {
		return transactions[i].Date < transactions[j].Date
	})

	// Track which dates we've seen
	dateSet := make(map[string]bool)

	for _, tx := range transactions {
		date := tx.Date
		dateSet[date] = true

		// Accumulate balances
		for _, posting := range tx.Postings {
			var amount float64
			if len(posting.Amount) > 0 {
				amount = convertAmount(posting.Amount[0].Quantity)
			}
			accountBalances[posting.Account] += amount
		}

		// Calculate and store net worth for this date
		var totalAssets float64
		var totalLiabilities float64

		for account, balance := range accountBalances {
			if strings.HasPrefix(account, "assets:") {
				totalAssets += balance
			} else if strings.HasPrefix(account, "liabilities:") {
				totalLiabilities += -balance
			}
		}

		netWorth := totalAssets - totalLiabilities
		dailyNetWorth[date] = math.Round(netWorth*100) / 100
	}

	// Get all unique dates and sort
	var dates []string
	for d := range dateSet {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	// Build result with dates in order
	var result []NetWorthPoint
	for _, date := range dates {
		result = append(result, NetWorthPoint{
			Date:     date,
			NetWorth: dailyNetWorth[date],
		})
	}

	return result, nil
}

// GetCategoryTrends returns spending trends for each category
func (p *Parser) GetCategoryTrends() ([]CategoryTrendData, error) {
	categorySpending, err := p.GetCategorySpending()
	if err != nil {
		return nil, err
	}

	// Group by tier instead of category
	tiers := make(map[string][]MonthAmountPair)
	for _, spending := range categorySpending {
		// Look up which tier this category belongs to
		tier := p.settings.GetTierForCategory(spending.Category)
		tierName := spending.Category // default to category name if not in any tier
		if tier != nil {
			tierName = tier.Name
		}

		// Find if we already have this month for this tier
		found := false
		for i, pair := range tiers[tierName] {
			if pair.Month == spending.Month {
				// Add to existing month total
				tiers[tierName][i].Amount += spending.Amount
				found = true
				break
			}
		}
		if !found {
			// New month for this tier
			tiers[tierName] = append(tiers[tierName], MonthAmountPair{
				Month:  spending.Month,
				Amount: spending.Amount,
			})
		}
	}

	// Build result
	var result []CategoryTrendData
	for tierName, data := range tiers {
		// Sort by month
		sort.Slice(data, func(i, j int) bool {
			return data[i].Month < data[j].Month
		})

		result = append(result, CategoryTrendData{
			Category: tierName,
			Data:     data,
		})
	}

	// Sort by tier name
	sort.Slice(result, func(i, j int) bool {
		return result[i].Category < result[j].Category
	})

	return result, nil
}

// extractSubcategory extracts subcategory from account path based on depth setting
// Example: "expenses:groceries:meat:beef" with depth=1 -> "groceries:meat"
// with depth=2 -> "groceries:meat:beef"
func (p *Parser) extractSubcategory(accountPath string, depth int) string {
	parts := strings.Split(accountPath, ":")

	// For expenses, skip first part (expenses)
	if strings.HasPrefix(accountPath, "expenses:") && len(parts) > 1 {
		endIndex := min(1+depth+1, len(parts))
		return strings.Join(parts[1:endIndex], ":")
	}

	// For other account types, include the type prefix
	endIndex := min(depth+1, len(parts))
	return strings.Join(parts[:endIndex], ":")
}

// GetCategoryDetail returns detailed data for a specific category
func (p *Parser) GetCategoryDetail(category string) (*CategoryDetailData, error) {
	transactions, err := p.GetTransactions()
	if err != nil {
		return nil, err
	}

	// Filter transactions for this category
	var filteredTxs []Transaction
	subcategoryTotals := make(map[string]float64)

	for _, tx := range transactions {
		hasCategory := false
		for _, posting := range tx.Postings {
			if !strings.HasPrefix(posting.Account, "expenses:") {
				continue
			}

			parts := strings.Split(posting.Account, ":")
			var postingCategory string
			if len(parts) >= 2 {
				postingCategory = parts[1]
			}

			if postingCategory == category {
				hasCategory = true

				// Extract subcategory based on depth
				subcategory := p.extractSubcategory(posting.Account, p.settings.SubcategoryDepth)

				var amount float64
				if len(posting.Amount) > 0 {
					amount = convertAmount(posting.Amount[0].Quantity)
				}
				if amount < 0 {
					amount = -amount
				}

				subcategoryTotals[subcategory] += amount
			}
		}

		if hasCategory {
			filteredTxs = append(filteredTxs, tx)
		}
	}

	// Build breakdown
	var breakdown []SubcategoryBreakdown
	for name, amount := range subcategoryTotals {
		breakdown = append(breakdown, SubcategoryBreakdown{
			Name:   name,
			Amount: amount,
		})
	}

	// Sort by amount descending
	sort.Slice(breakdown, func(i, j int) bool {
		return breakdown[i].Amount > breakdown[j].Amount
	})

	// Get budget history for this category
	budgetHistory, err := p.GetBudgetHistory()
	if err != nil {
		return nil, err
	}

	var categoryBudgetHistory []BudgetHistoryItem
	for _, item := range budgetHistory {
		if item.Category == category {
			categoryBudgetHistory = append(categoryBudgetHistory, item)
		}
	}

	return &CategoryDetailData{
		Category:      category,
		Transactions:  filteredTxs,
		BudgetHistory: categoryBudgetHistory,
		Breakdown:     breakdown,
	}, nil
}

// GetTierDetail returns detailed data for a specific tier
func (p *Parser) GetTierDetail(tierName string) (*TierDetailData, error) {
	// Find the tier
	var tier *config.Tier
	for i := range p.settings.Tiers {
		if p.settings.Tiers[i].Name == tierName {
			tier = &p.settings.Tiers[i]
			break
		}
	}

	if tier == nil {
		return nil, nil
	}

	transactions, err := p.GetTransactions()
	if err != nil {
		return nil, err
	}

	// Filter transactions for categories in this tier
	var filteredTxs []Transaction
	categoryTotals := make(map[string]float64)

	for _, tx := range transactions {
		hasTierCategory := false
		for _, posting := range tx.Postings {
			if !strings.HasPrefix(posting.Account, "expenses:") {
				continue
			}

			parts := strings.Split(posting.Account, ":")
			var category string
			if len(parts) >= 2 {
				category = parts[1]
			}

			// Check if category is in this tier
			for _, tierCat := range tier.Categories {
				if category == tierCat {
					hasTierCategory = true

					var amount float64
					if len(posting.Amount) > 0 {
						amount = convertAmount(posting.Amount[0].Quantity)
					}
					if amount < 0 {
						amount = -amount
					}

					categoryTotals[category] += amount
				}
			}
		}

		if hasTierCategory {
			filteredTxs = append(filteredTxs, tx)
		}
	}

	// Build breakdown by category (not subcategory for tiers)
	var breakdown []SubcategoryBreakdown
	for name, amount := range categoryTotals {
		breakdown = append(breakdown, SubcategoryBreakdown{
			Name:   name,
			Amount: amount,
		})
	}

	// Sort by amount descending
	sort.Slice(breakdown, func(i, j int) bool {
		return breakdown[i].Amount > breakdown[j].Amount
	})

	// Get budget history for all categories in this tier
	budgetHistory, err := p.GetBudgetHistory()
	if err != nil {
		return nil, err
	}

	var tierBudgetHistory []BudgetHistoryItem
	for _, item := range budgetHistory {
		for _, tierCat := range tier.Categories {
			if item.Category == tierCat {
				tierBudgetHistory = append(tierBudgetHistory, item)
				break
			}
		}
	}

	return &TierDetailData{
		Tier:          tierName,
		Transactions:  filteredTxs,
		BudgetHistory: tierBudgetHistory,
		Breakdown:     breakdown,
	}, nil
}

// GetAccountDetail returns detailed data for a specific account
func (p *Parser) GetAccountDetail(accountName string) (*AccountDetailData, error) {
	transactions, err := p.GetTransactions()
	if err != nil {
		return nil, err
	}

	// Filter transactions for this account
	var filteredTxs []Transaction
	balanceMap := make(map[string]float64)

	runningBalance := 0.0

	for _, tx := range transactions {
		hasAccount := false
		txAmount := 0.0

		for _, posting := range tx.Postings {
			if posting.Account == accountName {
				hasAccount = true

				var amount float64
				if len(posting.Amount) > 0 {
					amount = convertAmount(posting.Amount[0].Quantity)
				}
				txAmount += amount
			}
		}

		if hasAccount {
			filteredTxs = append(filteredTxs, tx)
			runningBalance += txAmount
			balanceMap[tx.Date] = runningBalance
		}
	}

	// Build balance history
	var balanceHistory []BalanceHistoryPoint
	for date, balance := range balanceMap {
		balanceHistory = append(balanceHistory, BalanceHistoryPoint{
			Date:    date,
			Balance: balance,
		})
	}

	// Sort by date
	sort.Slice(balanceHistory, func(i, j int) bool {
		return balanceHistory[i].Date < balanceHistory[j].Date
	})

	return &AccountDetailData{
		Account:        accountName,
		Transactions:   filteredTxs,
		BalanceHistory: balanceHistory,
	}, nil
}

// GetIncomeDetail returns detailed data for a specific income category
func (p *Parser) GetIncomeDetail(incomeName string) (*CategoryDetailData, error) {
	transactions, err := p.GetTransactions()
	if err != nil {
		return nil, err
	}

	// Filter transactions for this income category
	var filteredTxs []Transaction
	subcategoryTotals := make(map[string]float64)

	for _, tx := range transactions {
		hasIncome := false
		for _, posting := range tx.Postings {
			if !strings.HasPrefix(posting.Account, "income:") {
				continue
			}

			parts := strings.Split(posting.Account, ":")
			var postingIncome string
			if len(parts) >= 2 {
				postingIncome = parts[1]
			}

			if postingIncome == incomeName {
				hasIncome = true

				// Extract subcategory based on depth
				subcategory := p.extractSubcategory(posting.Account, p.settings.SubcategoryDepth)

				var amount float64
				if len(posting.Amount) > 0 {
					amount = convertAmount(posting.Amount[0].Quantity)
				}
				// For income, amounts are positive
				if amount < 0 {
					amount = -amount
				}

				subcategoryTotals[subcategory] += amount
			}
		}

		if hasIncome {
			filteredTxs = append(filteredTxs, tx)
		}
	}

	// Build breakdown
	var breakdown []SubcategoryBreakdown
	for name, amount := range subcategoryTotals {
		breakdown = append(breakdown, SubcategoryBreakdown{
			Name:   name,
			Amount: amount,
		})
	}

	// Sort by amount descending
	sort.Slice(breakdown, func(i, j int) bool {
		return breakdown[i].Amount > breakdown[j].Amount
	})

	return &CategoryDetailData{
		Category:      incomeName,
		Transactions:  filteredTxs,
		BudgetHistory: []BudgetHistoryItem{}, // Income doesn't have budget history like expenses
		Breakdown:     breakdown,
	}, nil
}

// GetYearOverYearComparison returns spending comparison for same months across years
func (p *Parser) GetYearOverYearComparison() ([]YearOverYearData, error) {
	categorySpending, err := p.GetCategorySpending()
	if err != nil {
		return nil, err
	}

	// Group by month (MM) and year
	// Map of "MM" -> year -> total spending
	monthComparison := make(map[string]map[string]float64)

	for _, spending := range categorySpending {
		// Extract month (MM) from YYYY-MM
		month := spending.Month[5:7] // Get "MM" part
		year := spending.Month[:4]   // Get "YYYY" part

		if monthComparison[month] == nil {
			monthComparison[month] = make(map[string]float64)
		}

		monthComparison[month][year] += spending.Amount
	}

	// Build result sorted by month
	var result []YearOverYearData
	for month := range monthComparison {
		result = append(result, YearOverYearData{
			Month: month,
			Years: monthComparison[month],
		})
	}

	// Sort by month
	sort.Slice(result, func(i, j int) bool {
		return result[i].Month < result[j].Month
	})

	return result, nil
}
