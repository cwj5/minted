package hledger

import (
	"encoding/json"
	"log"
	"math"
	"os/exec"
	"sort"
	"strings"
	"time"
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
	Category     string  `json:"category"`
	Average      float64 `json:"average"`
	CurrentMonth float64 `json:"currentMonth"`
	Variance     float64 `json:"variance"`
	PercentBudget float64 `json:"percentBudget"`
}

// Parser handles hledger journal parsing
type Parser struct {
	journalFile string
}

// NewParser creates a new hledger parser
func NewParser(journalFile string) *Parser {
	return &Parser{
		journalFile: journalFile,
	}
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
				if !strings.HasPrefix(name, "Assets:") && !strings.HasPrefix(name, "Liabilities:") {
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
func (p *Parser) GetTransactions(limit int) ([]Transaction, error) {
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

	// Limit results
	if limit > 0 && len(transactions) > limit {
		transactions = transactions[:limit]
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
	transactions, err := p.GetTransactions(0)
	if err != nil {
		return nil, err
	}

	// Map of month -> category -> total amount
	monthlyByCategory := make(map[string]map[string]float64)

	for _, tx := range transactions {
		month := getYearMonth(tx.Date)
		
		for _, posting := range tx.Postings {
			// Only include Expenses accounts
			if !strings.HasPrefix(posting.Account, "Expenses:") {
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
