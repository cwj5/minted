package dashboard

import (
	"net/http"

	"github.com/cwj5/minted/internal/config"
	"github.com/cwj5/minted/internal/hledger"
	"github.com/gin-gonic/gin"
)

// Service handles dashboard operations
type Service struct {
	parser   *hledger.Parser
	settings *config.Settings
}

// NewService creates a new dashboard service
func NewService(journalFile string, settings *config.Settings) *Service {
	return &Service{
		parser:   hledger.NewParser(journalFile, settings),
		settings: settings,
	}
}

// HandleIndex serves the main dashboard page
func (s *Service) HandleIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"title": "Minted - hledger Dashboard",
	})
}

// HandleSettings serves the settings page
func (s *Service) HandleSettings(c *gin.Context) {
	c.HTML(http.StatusOK, "settings.html", gin.H{
		"title": "Settings - Minted",
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
	transactions, err := s.parser.GetTransactions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, transactions)
}

// HandleSummary returns financial summary
func (s *Service) HandleSummary(c *gin.Context) {
	accounts, err := s.parser.GetAccounts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var totalAssets float64
	var totalLiabilities float64

	// Sum up assets and liabilities
	for _, account := range accounts {
		if account.Name[:7] == "assets:" {
			totalAssets += account.Balance
		} else if account.Name[:12] == "liabilities:" {
			// Liabilities in hledger are negative, convert to positive for display
			totalLiabilities += -account.Balance
		}
	}

	// Net worth is Assets - Liabilities
	netWorth := totalAssets - totalLiabilities

	c.JSON(http.StatusOK, gin.H{
		"totalAssets":      totalAssets,
		"totalLiabilities": totalLiabilities,
		"netWorth":         netWorth,
	})
}

// HandleBudgetComparison returns budget data with historical averages
func (s *Service) HandleBudgetComparison(c *gin.Context) {
	budgetItems, err := s.parser.GetBudgetData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, budgetItems)
}

// HandleMonthlyMetrics returns monthly income, expenses, and savings
func (s *Service) HandleMonthlyMetrics(c *gin.Context) {
	metrics, err := s.parser.GetMonthlyMetrics()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, metrics)
}

// HandleCategorySpending returns spending by category over time
func (s *Service) HandleCategorySpending(c *gin.Context) {
	spending, err := s.parser.GetCategorySpending()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, spending)
}

// HandleNetWorthOverTime returns net worth for each month
func (s *Service) HandleNetWorthOverTime(c *gin.Context) {
	data, err := s.parser.GetNetWorthOverTime()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

// HandleCategoryTrends returns spending trends for each category
func (s *Service) HandleCategoryTrends(c *gin.Context) {
	data, err := s.parser.GetCategoryTrends()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

// HandleYearOverYearComparison returns spending comparison across years
func (s *Service) HandleYearOverYearComparison(c *gin.Context) {
	data, err := s.parser.GetYearOverYearComparison()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

// HandleGetSettings returns the current application settings
func (s *Service) HandleGetSettings(c *gin.Context) {
	c.JSON(http.StatusOK, s.settings)
}

// HandleUpdateSettings updates application settings and saves to disk
func (s *Service) HandleUpdateSettings(c *gin.Context) {
	var updatedSettings config.Settings
	if err := c.BindJSON(&updatedSettings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid settings format"})
		return
	}

	// Update the settings in memory
	s.settings = &updatedSettings

	// Update the parser's settings so it uses the new tiers
	s.parser.UpdateSettings(&updatedSettings)

	// Save to disk
	if err := config.SaveSettings(&updatedSettings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "settings updated successfully"})
}
