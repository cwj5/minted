package dashboard

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/cwj5/minted/internal/config"
	"github.com/cwj5/minted/internal/hledger"
	"github.com/gin-gonic/gin"
)

// Service handles dashboard operations
type Service struct {
	parser          *hledger.Parser
	settings        *config.Settings
	cacheMu         sync.RWMutex
	cache           *CachedData
	cacheRefreshing bool
}

// SummaryData represents the summary response payload
type SummaryData struct {
	TotalAssets      float64 `json:"totalAssets"`
	TotalLiabilities float64 `json:"totalLiabilities"`
	NetWorth         float64 `json:"netWorth"`
}

// CachedData holds computed dashboard data for quick responses
type CachedData struct {
	Accounts         []hledger.Account
	Transactions     []hledger.Transaction
	Budget           []hledger.BudgetItem
	MonthlyMetrics   []hledger.MonthlyMetrics
	CategorySpending []hledger.CategorySpending
	NetWorthOverTime []hledger.NetWorthPoint
	CategoryTrends   []hledger.CategoryTrendData
	YearOverYear     []hledger.YearOverYearData
	Summary          SummaryData
	LastRefresh      time.Time
	Stale            bool
}

// NewService creates a new dashboard service
func NewService(journalFile string, settings *config.Settings) *Service {
	s := &Service{
		parser:   hledger.NewParser(journalFile, settings),
		settings: settings,
	}

	// Warm the cache at startup (best effort)
	if err := s.RebuildCache(); err != nil {
		// Keep running; handlers will return a refresh-needed message until cache succeeds
	}

	return s
}

// RebuildCache refreshes all dashboard data in a single pass.
func (s *Service) RebuildCache() error {
	s.cacheMu.Lock()
	if s.cacheRefreshing {
		s.cacheMu.Unlock()
		return errors.New("refresh already in progress")
	}
	s.cacheRefreshing = true
	s.cacheMu.Unlock()

	defer func() {
		s.cacheMu.Lock()
		s.cacheRefreshing = false
		s.cacheMu.Unlock()
	}()

	accounts, err := s.parser.GetAccounts()
	if err != nil {
		return err
	}

	summary := SummaryData{}
	for _, account := range accounts {
		if len(account.Name) >= 7 && account.Name[:7] == "assets:" {
			summary.TotalAssets += account.Balance
		} else if len(account.Name) >= 12 && account.Name[:12] == "liabilities:" {
			// Liabilities in hledger are negative; convert to positive
			summary.TotalLiabilities += -account.Balance
		}
	}
	summary.NetWorth = summary.TotalAssets - summary.TotalLiabilities

	transactions, err := s.parser.GetTransactions()
	if err != nil {
		return err
	}

	budgetItems, err := s.parser.GetBudgetData()
	if err != nil {
		return err
	}

	monthlyMetrics, err := s.parser.GetMonthlyMetrics()
	if err != nil {
		return err
	}

	categorySpending, err := s.parser.GetCategorySpending()
	if err != nil {
		return err
	}

	netWorth, err := s.parser.GetNetWorthOverTime()
	if err != nil {
		return err
	}

	categoryTrends, err := s.parser.GetCategoryTrends()
	if err != nil {
		return err
	}

	yearOverYear, err := s.parser.GetYearOverYearComparison()
	if err != nil {
		return err
	}

	newCache := &CachedData{
		Accounts:         accounts,
		Transactions:     transactions,
		Budget:           budgetItems,
		MonthlyMetrics:   monthlyMetrics,
		CategorySpending: categorySpending,
		NetWorthOverTime: netWorth,
		CategoryTrends:   categoryTrends,
		YearOverYear:     yearOverYear,
		Summary:          summary,
		LastRefresh:      time.Now(),
		Stale:            false,
	}

	s.cacheMu.Lock()
	s.cache = newCache
	s.cacheMu.Unlock()

	return nil
}

// getCache safely returns the cached data
func (s *Service) getCache() (*CachedData, bool) {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	if s.cache == nil {
		return nil, false
	}
	return s.cache, true
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
	cache, ok := s.getCache()
	if !ok {
		c.JSON(http.StatusAccepted, gin.H{"message": "cache empty; refresh required", "needsRefresh": true})
		return
	}
	c.JSON(http.StatusOK, cache.Accounts)
}

// HandleTransactions returns transaction data as JSON
func (s *Service) HandleTransactions(c *gin.Context) {
	cache, ok := s.getCache()
	if !ok {
		c.JSON(http.StatusAccepted, gin.H{"message": "cache empty; refresh required", "needsRefresh": true})
		return
	}
	c.JSON(http.StatusOK, cache.Transactions)
}

// HandleSummary returns financial summary
func (s *Service) HandleSummary(c *gin.Context) {
	cache, ok := s.getCache()
	if !ok {
		c.JSON(http.StatusAccepted, gin.H{"message": "cache empty; refresh required", "needsRefresh": true})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"totalAssets":      cache.Summary.TotalAssets,
		"totalLiabilities": cache.Summary.TotalLiabilities,
		"netWorth":         cache.Summary.NetWorth,
	})
}

// HandleBudgetComparison returns budget data with historical averages
func (s *Service) HandleBudgetComparison(c *gin.Context) {
	cache, ok := s.getCache()
	if !ok {
		c.JSON(http.StatusAccepted, gin.H{"message": "cache empty; refresh required", "needsRefresh": true})
		return
	}
	c.JSON(http.StatusOK, cache.Budget)
}

// HandleMonthlyMetrics returns monthly income, expenses, and savings
func (s *Service) HandleMonthlyMetrics(c *gin.Context) {
	cache, ok := s.getCache()
	if !ok {
		c.JSON(http.StatusAccepted, gin.H{"message": "cache empty; refresh required", "needsRefresh": true})
		return
	}
	c.JSON(http.StatusOK, cache.MonthlyMetrics)
}

// HandleCategorySpending returns spending by category over time
func (s *Service) HandleCategorySpending(c *gin.Context) {
	cache, ok := s.getCache()
	if !ok {
		c.JSON(http.StatusAccepted, gin.H{"message": "cache empty; refresh required", "needsRefresh": true})
		return
	}
	c.JSON(http.StatusOK, cache.CategorySpending)
}

// HandleNetWorthOverTime returns net worth for each month
func (s *Service) HandleNetWorthOverTime(c *gin.Context) {
	cache, ok := s.getCache()
	if !ok {
		c.JSON(http.StatusAccepted, gin.H{"message": "cache empty; refresh required", "needsRefresh": true})
		return
	}
	c.JSON(http.StatusOK, cache.NetWorthOverTime)
}

// HandleCategoryTrends returns spending trends for each category
func (s *Service) HandleCategoryTrends(c *gin.Context) {
	cache, ok := s.getCache()
	if !ok {
		c.JSON(http.StatusAccepted, gin.H{"message": "cache empty; refresh required", "needsRefresh": true})
		return
	}
	c.JSON(http.StatusOK, cache.CategoryTrends)
}

// HandleYearOverYearComparison returns spending comparison across years
func (s *Service) HandleYearOverYearComparison(c *gin.Context) {
	cache, ok := s.getCache()
	if !ok {
		c.JSON(http.StatusAccepted, gin.H{"message": "cache empty; refresh required", "needsRefresh": true})
		return
	}
	c.JSON(http.StatusOK, cache.YearOverYear)
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

	// Mark cache as stale so the next refresh will recompute with new settings
	s.cacheMu.Lock()
	if s.cache != nil {
		s.cache.Stale = true
	}
	s.cacheMu.Unlock()

	// Save to disk
	if err := config.SaveSettings(&updatedSettings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "settings updated successfully"})
}

// HandleCacheStatus returns cache metadata
func (s *Service) HandleCacheStatus(c *gin.Context) {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()

	if s.cache == nil {
		c.JSON(http.StatusOK, gin.H{
			"hasCache":     false,
			"inProgress":   s.cacheRefreshing,
			"lastRefresh":  nil,
			"stale":        false,
			"needsRefresh": true,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"hasCache":    true,
		"inProgress":  s.cacheRefreshing,
		"lastRefresh": s.cache.LastRefresh,
		"stale":       s.cache.Stale,
	})
}

// HandleCacheRefresh triggers a rebuild of cached data
func (s *Service) HandleCacheRefresh(c *gin.Context) {
	if err := s.RebuildCache(); err != nil {
		if err.Error() == "refresh already in progress" {
			c.JSON(http.StatusAccepted, gin.H{"message": "refresh already in progress", "inProgress": true})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "cache rebuilt", "lastRefresh": time.Now()})
}
