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

// Stub filtered methods that delegate to existing methods
// These can be enhanced later to properly filter their specific data

func (p *Parser) GetBudgetHistoryFiltered(startDate, endDate string) ([]BudgetHistoryItem, error) {
	return p.GetBudgetHistory()
}

func (p *Parser) GetMonthlyMetricsFiltered(startDate, endDate string) ([]MonthlyMetrics, error) {
	return p.GetMonthlyMetrics()
}

func (p *Parser) GetCategorySpendingFiltered(startDate, endDate string) ([]CategorySpending, error) {
	return p.GetCategorySpending()
}

func (p *Parser) GetIncomeBreakdownFiltered(startDate, endDate string) ([]CategorySpending, error) {
	return p.GetIncomeBreakdown()
}

func (p *Parser) GetIncomeHistoryFiltered(startDate, endDate string) ([]BudgetHistoryItem, error) {
	return p.GetIncomeHistory()
}

func (p *Parser) GetNetWorthOverTimeFiltered(startDate, endDate string) ([]NetWorthPoint, error) {
	return p.GetNetWorthOverTime()
}

func (p *Parser) GetCategoryTrendsFiltered(startDate, endDate string) ([]CategoryTrendData, error) {
	return p.GetCategoryTrends()
}

func (p *Parser) GetYearOverYearComparisonFiltered(startDate, endDate string) ([]YearOverYearData, error) {
	return p.GetYearOverYearComparison()
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
