package dashboard

import (
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/cwj5/minted/internal/config"
	"github.com/cwj5/minted/internal/hledger"
	"github.com/gin-gonic/gin"
)

// DateFilter holds start and end dates for filtering
type DateFilter struct {
	StartDate string
	EndDate   string
}

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
	BudgetHistory    []hledger.BudgetHistoryItem
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

	budgetHistory, err := s.parser.GetBudgetHistory()
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
		BudgetHistory:    budgetHistory,
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

// getDateFilter extracts and validates date filter parameters from request
func (s *Service) getDateFilter(c *gin.Context) *DateFilter {
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	// Only return a filter if both dates are provided
	if startDate != "" && endDate != "" {
		return &DateFilter{
			StartDate: startDate,
			EndDate:   endDate,
		}
	}

	return nil
}

// hasDateFilter checks if date filtering is active
func (s *Service) hasDateFilter(c *gin.Context) bool {
	return c.Query("startDate") != "" && c.Query("endDate") != ""
}

// HandleIndex serves the main dashboard page
func (s *Service) HandleIndex(c *gin.Context) {
	// Check if this is a detail page request
	page := c.Query("page")
	if page == "detail" {
		c.HTML(http.StatusOK, "detail.html", gin.H{
			"title": "Detail - Minted",
		})
		return
	}

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
// HandleAccounts returns account data as JSON
func (s *Service) HandleAccounts(c *gin.Context) {
	// Check if date filtering is requested
	if s.hasDateFilter(c) {
		filter := s.getDateFilter(c)
		accounts, err := s.parser.GetAccountsFiltered(filter.StartDate, filter.EndDate)
		if err != nil {
			log.Printf("Error getting filtered accounts: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get accounts"})
			return
		}
		c.JSON(http.StatusOK, accounts)
		return
	}

	// Use cache for unfiltered requests
	cache, ok := s.getCache()
	if !ok {
		c.JSON(http.StatusAccepted, gin.H{"message": "cache empty; refresh required", "needsRefresh": true})
		return
	}
	c.JSON(http.StatusOK, cache.Accounts)
}

// HandleTransactions returns transaction data as JSON
func (s *Service) HandleTransactions(c *gin.Context) {
	// Check if date filtering is requested
	if s.hasDateFilter(c) {
		filter := s.getDateFilter(c)
		transactions, err := s.parser.GetTransactionsFiltered(filter.StartDate, filter.EndDate)
		if err != nil {
			log.Printf("Error getting filtered transactions: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get transactions"})
			return
		}
		c.JSON(http.StatusOK, transactions)
		return
	}

	// Use cache for unfiltered requests
	cache, ok := s.getCache()
	if !ok {
		c.JSON(http.StatusAccepted, gin.H{"message": "cache empty; refresh required", "needsRefresh": true})
		return
	}
	c.JSON(http.StatusOK, cache.Transactions)
}

// HandleSummary returns financial summary
func (s *Service) HandleSummary(c *gin.Context) {
	// Check if date filtering is requested
	if s.hasDateFilter(c) {
		filter := s.getDateFilter(c)
		accounts, err := s.parser.GetAccountsFiltered(filter.StartDate, filter.EndDate)
		if err != nil {
			log.Printf("Error getting filtered accounts: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get summary"})
			return
		}

		summary := SummaryData{}
		for _, account := range accounts {
			if len(account.Name) >= 7 && account.Name[:7] == "assets:" {
				summary.TotalAssets += account.Balance
			} else if len(account.Name) >= 12 && account.Name[:12] == "liabilities:" {
				summary.TotalLiabilities += -account.Balance
			}
		}
		summary.NetWorth = summary.TotalAssets - summary.TotalLiabilities

		c.JSON(http.StatusOK, gin.H{
			"totalAssets":      summary.TotalAssets,
			"totalLiabilities": summary.TotalLiabilities,
			"netWorth":         summary.NetWorth,
		})
		return
	}

	// Use cache for unfiltered requests
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

// HandleBudgetHistory returns historical budget vs actuals
func (s *Service) HandleBudgetHistory(c *gin.Context) {
	// Check if date filtering is requested
	if s.hasDateFilter(c) {
		filter := s.getDateFilter(c)
		budgetHistory, err := s.parser.GetBudgetHistoryFiltered(filter.StartDate, filter.EndDate)
		if err != nil {
			log.Printf("Error getting filtered budget history: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get budget history"})
			return
		}
		c.JSON(http.StatusOK, budgetHistory)
		return
	}

	// Use cache for unfiltered requests
	cache, ok := s.getCache()
	if !ok {
		c.JSON(http.StatusAccepted, gin.H{"message": "cache empty; refresh required", "needsRefresh": true})
		return
	}
	c.JSON(http.StatusOK, cache.BudgetHistory)
}

// HandleMonthlyMetrics returns monthly income, expenses, and savings
func (s *Service) HandleMonthlyMetrics(c *gin.Context) {
	// Check if date filtering is requested
	if s.hasDateFilter(c) {
		filter := s.getDateFilter(c)
		monthlyMetrics, err := s.parser.GetMonthlyMetricsFiltered(filter.StartDate, filter.EndDate)
		if err != nil {
			log.Printf("Error getting filtered monthly metrics: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get monthly metrics"})
			return
		}
		c.JSON(http.StatusOK, monthlyMetrics)
		return
	}

	// Use cache for unfiltered requests
	cache, ok := s.getCache()
	if !ok {
		c.JSON(http.StatusAccepted, gin.H{"message": "cache empty; refresh required", "needsRefresh": true})
		return
	}
	c.JSON(http.StatusOK, cache.MonthlyMetrics)
}

// HandleCategorySpending returns spending by category over time
func (s *Service) HandleCategorySpending(c *gin.Context) {
	// Check if date filtering is requested
	if s.hasDateFilter(c) {
		filter := s.getDateFilter(c)
		categorySpending, err := s.parser.GetCategorySpendingFiltered(filter.StartDate, filter.EndDate)
		if err != nil {
			log.Printf("Error getting filtered category spending: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get category spending"})
			return
		}
		c.JSON(http.StatusOK, categorySpending)
		return
	}

	// Use cache for unfiltered requests
	cache, ok := s.getCache()
	if !ok {
		c.JSON(http.StatusAccepted, gin.H{"message": "cache empty; refresh required", "needsRefresh": true})
		return
	}
	c.JSON(http.StatusOK, cache.CategorySpending)
}

// HandleIncomeBreakdown returns income categories aggregated across all months
func (s *Service) HandleIncomeBreakdown(c *gin.Context) {
	if s.hasDateFilter(c) {
		filter := s.getDateFilter(c)
		incomeBreakdown, err := s.parser.GetIncomeBreakdownFiltered(filter.StartDate, filter.EndDate)
		if err != nil {
			log.Printf("Error getting filtered income breakdown: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get income breakdown"})
			return
		}
		c.JSON(http.StatusOK, incomeBreakdown)
		return
	}

	incomeBreakdown, err := s.parser.GetIncomeBreakdown()
	if err != nil {
		log.Printf("Error getting income breakdown: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get income breakdown"})
		return
	}
	c.JSON(http.StatusOK, incomeBreakdown)
}

// HandleIncomeHistory returns income history by category and month
func (s *Service) HandleIncomeHistory(c *gin.Context) {
	if s.hasDateFilter(c) {
		filter := s.getDateFilter(c)
		incomeHistory, err := s.parser.GetIncomeHistoryFiltered(filter.StartDate, filter.EndDate)
		if err != nil {
			log.Printf("Error getting filtered income history: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get income history"})
			return
		}
		c.JSON(http.StatusOK, incomeHistory)
		return
	}

	incomeHistory, err := s.parser.GetIncomeHistory()
	if err != nil {
		log.Printf("Error getting income history: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get income history"})
		return
	}
	c.JSON(http.StatusOK, incomeHistory)
}

// HandleNetWorthOverTime returns net worth for each month
func (s *Service) HandleNetWorthOverTime(c *gin.Context) {
	// Check if date filtering is requested
	if s.hasDateFilter(c) {
		filter := s.getDateFilter(c)
		netWorth, err := s.parser.GetNetWorthOverTimeFiltered(filter.StartDate, filter.EndDate)
		if err != nil {
			log.Printf("Error getting filtered net worth: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get net worth"})
			return
		}
		c.JSON(http.StatusOK, netWorth)
		return
	}

	// Use cache for unfiltered requests
	cache, ok := s.getCache()
	if !ok {
		c.JSON(http.StatusAccepted, gin.H{"message": "cache empty; refresh required", "needsRefresh": true})
		return
	}
	c.JSON(http.StatusOK, cache.NetWorthOverTime)
}

// HandleCategoryTrends returns spending trends for each category
func (s *Service) HandleCategoryTrends(c *gin.Context) {
	// Check if date filtering is requested
	if s.hasDateFilter(c) {
		filter := s.getDateFilter(c)
		categoryTrends, err := s.parser.GetCategoryTrendsFiltered(filter.StartDate, filter.EndDate)
		if err != nil {
			log.Printf("Error getting filtered category trends: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get category trends"})
			return
		}
		c.JSON(http.StatusOK, categoryTrends)
		return
	}

	// Use cache for unfiltered requests
	cache, ok := s.getCache()
	if !ok {
		c.JSON(http.StatusAccepted, gin.H{"message": "cache empty; refresh required", "needsRefresh": true})
		return
	}
	c.JSON(http.StatusOK, cache.CategoryTrends)
}

// HandleYearOverYearComparison returns spending comparison across years
func (s *Service) HandleYearOverYearComparison(c *gin.Context) {
	// Check if date filtering is requested
	if s.hasDateFilter(c) {
		filter := s.getDateFilter(c)
		yoyData, err := s.parser.GetYearOverYearComparisonFiltered(filter.StartDate, filter.EndDate)
		if err != nil {
			log.Printf("Error getting filtered year-over-year: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get year-over-year comparison"})
			return
		}
		c.JSON(http.StatusOK, yoyData)
		return
	}

	// Use cache for unfiltered requests
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

// HandleCategoryDetail returns detailed view for a specific category
func (s *Service) HandleCategoryDetail(c *gin.Context) {
	category := c.Query("category")
	if category == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category parameter required"})
		return
	}

	var detail interface{}
	var err error

	if s.hasDateFilter(c) {
		filter := s.getDateFilter(c)
		detail, err = s.parser.GetCategoryDetailFiltered(category, filter.StartDate, filter.EndDate)
	} else {
		detail, err = s.parser.GetCategoryDetail(category)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, detail)
}

// HandleTierDetail returns detailed view for a specific tier
func (s *Service) HandleTierDetail(c *gin.Context) {
	tier := c.Query("tier")
	if tier == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tier parameter required"})
		return
	}

	var detail interface{}
	var err error

	if s.hasDateFilter(c) {
		filter := s.getDateFilter(c)
		detail, err = s.parser.GetTierDetailFiltered(tier, filter.StartDate, filter.EndDate)
	} else {
		detail, err = s.parser.GetTierDetail(tier)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if detail == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tier not found"})
		return
	}

	c.JSON(http.StatusOK, detail)
}

// HandleAccountDetail returns detailed view for a specific account
func (s *Service) HandleAccountDetail(c *gin.Context) {
	account := c.Query("account")
	if account == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "account parameter required"})
		return
	}

	var detail interface{}
	var err error

	if s.hasDateFilter(c) {
		filter := s.getDateFilter(c)
		detail, err = s.parser.GetAccountDetailFiltered(account, filter.StartDate, filter.EndDate)
	} else {
		detail, err = s.parser.GetAccountDetail(account)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, detail)
}

// HandleIncomeDetail returns detailed view for a specific income category
func (s *Service) HandleIncomeDetail(c *gin.Context) {
	income := c.Query("income")
	if income == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "income parameter required"})
		return
	}

	var detail interface{}
	var err error

	if s.hasDateFilter(c) {
		filter := s.getDateFilter(c)
		detail, err = s.parser.GetIncomeDetailFiltered(income, filter.StartDate, filter.EndDate)
	} else {
		detail, err = s.parser.GetIncomeDetail(income)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, detail)
}
