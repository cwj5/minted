package hledger

import (
	"encoding/json"
	"log"
	"math"
	"os/exec"
	"sort"
	"strings"

	"github.com/cwj5/minted/internal/config"
)

// Filtered method implementations - these bypass the cache and apply date ranges

// GetAccountsFiltered retrieves accounts with balances as of the end date
func (p *Parser) GetAccountsFiltered(startDate, endDate string) ([]Account, error) {
	args := []string{"-f", p.journalFile, "balance", "--empty", "-O", "json"}
	args = append(args, p.buildDateArgs(startDate, endDate)...)

	cmd := exec.Command("hledger", args...)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Error running hledger balance (filtered): file=%s, error=%v", p.journalFile, err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			log.Printf("stderr: %s", string(exitErr.Stderr))
		}
		return nil, err
	}

	var balanceData [][]interface{}
	err = json.Unmarshal(output, &balanceData)
	if err != nil {
		log.Printf("Error parsing JSON: %v", err)
		return nil, err
	}

	var accounts []Account

	if len(balanceData) > 0 {
		accountsList := balanceData[0]

		for _, item := range accountsList {
			if itemArr, ok := item.([]interface{}); ok && len(itemArr) >= 4 {
				name, ok := itemArr[0].(string)
				if !ok || name == "" {
					continue
				}

				commodityData, ok := itemArr[3].([]interface{})
				if !ok || len(commodityData) == 0 {
					accounts = append(accounts, Account{
						Name:     name,
						Balance:  0,
						Currency: "",
					})
					continue
				}

				firstAmount, ok := commodityData[0].(map[string]interface{})
				if !ok {
					continue
				}

				quantityData, ok := firstAmount["aquantity"].(map[string]interface{})
				if !ok {
					continue
				}

				mantissa, _ := quantityData["decimalMantissa"].(float64)
				places, _ := quantityData["decimalPlaces"].(float64)
				balance := mantissa / math.Pow(10, places)

				currency, _ := firstAmount["acommodity"].(string)

				accounts = append(accounts, Account{
					Name:     name,
					Balance:  balance,
					Currency: currency,
				})
			}
		}
	}

	return accounts, nil
}

// GetTransactionsFiltered retrieves transactions within a date range
func (p *Parser) GetTransactionsFiltered(startDate, endDate string) ([]Transaction, error) {
	args := []string{"-f", p.journalFile, "print", "-O", "json"}
	args = append(args, p.buildDateArgs(startDate, endDate)...)

	cmd := exec.Command("hledger", args...)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Error running hledger print (filtered): %v", err)
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

// GetMonthlyMetricsFiltered returns financial metrics filtered to a specific date range
func (p *Parser) GetMonthlyMetricsFiltered(startDate, endDate string) ([]MonthlyMetrics, error) {
	transactions, err := p.GetTransactionsFiltered(startDate, endDate)
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

		// Calculate savings rate
		savingsRate := 0.0
		if data.income > 0 {
			savingsRate = ((data.income - data.expenses) / data.income) * 100
		}

		metrics = append(metrics, MonthlyMetrics{
			Month:       month,
			Income:      math.Round(data.income*100) / 100,
			Expenses:    math.Round(data.expenses*100) / 100,
			NetWorth:    0.0, // Simplified
			SavingsRate: math.Round(savingsRate*100) / 100,
		})
	}

	return metrics, nil
}

// GetCategorySpendingFiltered returns category spending filtered to a specific date range
func (p *Parser) GetCategorySpendingFiltered(startDate, endDate string) ([]CategorySpending, error) {
	transactions, err := p.GetTransactionsFiltered(startDate, endDate)
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

// GetIncomeBreakdownFiltered returns income categories aggregated within a date range
func (p *Parser) GetIncomeBreakdownFiltered(startDate, endDate string) ([]CategorySpending, error) {
	transactions, err := p.GetTransactionsFiltered(startDate, endDate)
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

	// Build result - return as if all income happened in a single period
	var result []CategorySpending
	for category, amount := range incomeCategories {
		result = append(result, CategorySpending{
			Month:    "period",
			Category: category,
			Amount:   math.Round(amount*100) / 100,
		})
	}

	// Sort by category
	sort.Slice(result, func(i, j int) bool {
		return result[i].Category < result[j].Category
	})

	return result, nil
}

// GetBudgetHistoryFiltered returns budget history filtered to a specific date range
func (p *Parser) GetBudgetHistoryFiltered(startDate, endDate string) ([]BudgetHistoryItem, error) {
	transactions, err := p.GetTransactionsFiltered(startDate, endDate)
	if err != nil {
		return nil, err
	}

	// Map of month -> category -> amount
	monthlySpending := make(map[string]map[string]float64)

	for _, tx := range transactions {
		month := getYearMonth(tx.Date)

		for _, posting := range tx.Postings {
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

			if monthlySpending[month] == nil {
				monthlySpending[month] = make(map[string]float64)
			}
			monthlySpending[month][category] += amount
		}
	}

	// Collect all months
	var allMonths []string
	for month := range monthlySpending {
		allMonths = append(allMonths, month)
	}
	sort.Strings(allMonths)

	// Build category history
	categoryHistory := make(map[string][]float64)
	for _, categories := range monthlySpending {
		for category, amount := range categories {
			categoryHistory[category] = append(categoryHistory[category], amount)
		}
	}

	var history []BudgetHistoryItem

	for category, amounts := range categoryHistory {
		if len(amounts) < 1 {
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

// GetIncomeHistoryFiltered returns income history filtered to a specific date range
func (p *Parser) GetIncomeHistoryFiltered(startDate, endDate string) ([]BudgetHistoryItem, error) {
	transactions, err := p.GetTransactionsFiltered(startDate, endDate)
	if err != nil {
		return nil, err
	}

	// Map of month -> category -> amount
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

	// Build category history
	categoryHistory := make(map[string][]float64)
	for _, categories := range monthlyIncome {
		for category, amount := range categories {
			categoryHistory[category] = append(categoryHistory[category], amount)
		}
	}

	var history []BudgetHistoryItem

	for category, amounts := range categoryHistory {
		if len(amounts) < 1 {
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
				OverBudget:      false, // Income doesn't have "over budget"
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

// GetNetWorthOverTimeFiltered returns net worth points filtered to a specific date range
func (p *Parser) GetNetWorthOverTimeFiltered(startDate, endDate string) ([]NetWorthPoint, error) {
	transactions, err := p.GetTransactionsFiltered(startDate, endDate)
	if err != nil {
		return nil, err
	}

	// Map of date -> net worth
	dateNetWorth := make(map[string]float64)

	for _, tx := range transactions {
		for _, posting := range tx.Postings {
			// Include all asset/liability accounts to calculate net worth
			if strings.HasPrefix(posting.Account, "assets:") || strings.HasPrefix(posting.Account, "liabilities:") {
				var amount float64
				if len(posting.Amount) > 0 {
					amount = convertAmount(posting.Amount[0].Quantity)
				}

				// Liabilities are negative
				if strings.HasPrefix(posting.Account, "liabilities:") {
					amount = -amount
				}

				dateNetWorth[tx.Date] += amount
			}
		}
	}

	// Build result
	var result []NetWorthPoint
	for date, netWorth := range dateNetWorth {
		result = append(result, NetWorthPoint{
			Date:     date,
			NetWorth: math.Round(netWorth*100) / 100,
		})
	}

	// Sort by date
	sort.Slice(result, func(i, j int) bool {
		return result[i].Date < result[j].Date
	})

	return result, nil
}

// GetCategoryTrendsFiltered returns category spending trends filtered to a specific date range
func (p *Parser) GetCategoryTrendsFiltered(startDate, endDate string) ([]CategoryTrendData, error) {
	spending, err := p.GetCategorySpendingFiltered(startDate, endDate)
	if err != nil {
		return nil, err
	}

	// Map of category -> list of month amounts
	categoryData := make(map[string][]MonthAmountPair)

	for _, item := range spending {
		categoryData[item.Category] = append(categoryData[item.Category], MonthAmountPair{
			Month:  item.Month,
			Amount: item.Amount,
		})
	}

	// Build result
	var result []CategoryTrendData
	for category, data := range categoryData {
		// Sort by month
		sort.Slice(data, func(i, j int) bool {
			return data[i].Month < data[j].Month
		})

		result = append(result, CategoryTrendData{
			Category: category,
			Data:     data,
		})
	}

	// Sort by category
	sort.Slice(result, func(i, j int) bool {
		return result[i].Category < result[j].Category
	})

	return result, nil
}

// GetYearOverYearComparisonFiltered returns YoY data filtered to a specific date range
func (p *Parser) GetYearOverYearComparisonFiltered(startDate, endDate string) ([]YearOverYearData, error) {
	spending, err := p.GetCategorySpendingFiltered(startDate, endDate)
	if err != nil {
		return nil, err
	}

	// Map of month (MM) -> year (YYYY) -> amount
	monthYearData := make(map[string]map[string]float64)

	for _, item := range spending {
		// Extract month and year from item.Month (format: YYYY-MM)
		if len(item.Month) < 7 {
			continue
		}

		month := item.Month[5:7] // Get MM part
		year := item.Month[:4]    // Get YYYY part

		if monthYearData[month] == nil {
			monthYearData[month] = make(map[string]float64)
		}

		monthYearData[month][year] += item.Amount
	}

	// Build result
	var result []YearOverYearData
	for month, years := range monthYearData {
		result = append(result, YearOverYearData{
			Month: month,
			Years: years,
		})
	}

	// Sort by month
	sort.Slice(result, func(i, j int) bool {
		return result[i].Month < result[j].Month
	})

	return result, nil
}

// Detail page filtered methods

func (p *Parser) GetCategoryDetailFiltered(category, startDate, endDate string) (*CategoryDetailData, error) {
	transactions, err := p.GetTransactionsFiltered(startDate, endDate)
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

	// Get budget history - filtering not fully implemented yet, using all history
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

func (p *Parser) GetTierDetailFiltered(tier, startDate, endDate string) (*TierDetailData, error) {
	// Find the tier
	var tierConfig *config.Tier
	for i := range p.settings.Tiers {
		if p.settings.Tiers[i].Name == tier {
			tierConfig = &p.settings.Tiers[i]
			break
		}
	}

	if tierConfig == nil {
		return nil, nil
	}

	transactions, err := p.GetTransactionsFiltered(startDate, endDate)
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
			for _, tierCat := range tierConfig.Categories {
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
		for _, tierCat := range tierConfig.Categories {
			if item.Category == tierCat {
				tierBudgetHistory = append(tierBudgetHistory, item)
				break
			}
		}
	}

	return &TierDetailData{
		Tier:          tier,
		Transactions:  filteredTxs,
		BudgetHistory: tierBudgetHistory,
		Breakdown:     breakdown,
	}, nil
}

func (p *Parser) GetAccountDetailFiltered(account, startDate, endDate string) (*AccountDetailData, error) {
	transactions, err := p.GetTransactionsFiltered(startDate, endDate)
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
			if posting.Account == account {
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
		Account:        account,
		Transactions:   filteredTxs,
		BalanceHistory: balanceHistory,
	}, nil
}

func (p *Parser) GetIncomeDetailFiltered(incomeName, startDate, endDate string) (*CategoryDetailData, error) {
	transactions, err := p.GetTransactionsFiltered(startDate, endDate)
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
