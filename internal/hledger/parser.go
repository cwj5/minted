package hledger

import (
	"encoding/json"
	"log"
	"os/exec"
	"strings"
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
