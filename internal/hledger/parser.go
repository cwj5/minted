package hledger

import (
	"bufio"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Account represents an hledger account
type Account struct {
	Name     string  `json:"name"`
	Balance  float64 `json:"balance"`
	Currency string  `json:"currency"`
}

// Transaction represents a transaction
type Transaction struct {
	Date        time.Time `json:"date"`
	Description string    `json:"description"`
	Postings    []Posting `json:"postings"`
}

// Posting represents a posting within a transaction
type Posting struct {
	Account  string  `json:"account"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
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

// GetAccounts retrieves all accounts from hledger
func (p *Parser) GetAccounts() ([]Account, error) {
	cmd := exec.Command("hledger", "-f", p.journalFile, "accounts", "--tree")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Error running hledger: %v", err)
		return nil, err
	}

	var accounts []Account
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			// TODO: Parse balances if needed
			accounts = append(accounts, Account{
				Name:     line,
				Currency: "USD",
			})
		}
	}

	return accounts, nil
}

// GetTransactions retrieves recent transactions
func (p *Parser) GetTransactions(limit int) ([]Transaction, error) {
	cmd := exec.Command("hledger", "-f", p.journalFile, "register", "--limit", string(rune(limit)))
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Error running hledger: %v", err)
		return nil, err
	}

	var transactions []Transaction
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			// TODO: Parse transaction details
			_ = line
		}
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

	// TODO: Parse JSON output and extract balance
	_ = output

	return 0, nil
}
