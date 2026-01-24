package dashboard

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/minted/internal/hledger"
)

// Service handles dashboard operations
type Service struct {
	parser *hledger.Parser
}

// NewService creates a new dashboard service
func NewService(journalFile string) *Service {
	return &Service{
		parser: hledger.NewParser(journalFile),
	}
}

// HandleIndex serves the main dashboard page
func (s *Service) HandleIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"title": "Minted - hledger Dashboard",
	})
}

// HandleAccounts returns account data as JSON
func (s *Service) HandleAccounts(c *gin.Context) {
	accounts, err := s.parser.GetAccounts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, accounts)
}

// HandleTransactions returns transaction data as JSON
func (s *Service) HandleTransactions(c *gin.Context) {
	limit := 50
	transactions, err := s.parser.GetTransactions(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, transactions)
}

// HandleSummary returns financial summary
func (s *Service) HandleSummary(c *gin.Context) {
	// TODO: Implement summary calculation
	c.JSON(http.StatusOK, gin.H{
		"totalAssets":     0,
		"totalLiabilities": 0,
		"netWorth":        0,
	})
}
