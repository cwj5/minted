// Format currency
function formatCurrency(amount) {
    return new Intl.NumberFormat('en-US', {
        style: 'currency',
        currency: 'USD'
    }).format(amount);
}

// Date filter utilities
function getURLParams() {
    const params = new URLSearchParams(window.location.search);
    return {
        startDate: params.get('startDate') || '',
        endDate: params.get('endDate') || '',
        preset: params.get('preset') || 'all'
    };
}

function setURLParams(startDate, endDate, preset) {
    const params = new URLSearchParams(window.location.search);

    if (preset && preset !== 'all') {
        params.set('preset', preset);
        params.delete('startDate');
        params.delete('endDate');
    } else if (startDate && endDate) {
        params.set('startDate', startDate);
        params.set('endDate', endDate);
        params.delete('preset');
    } else {
        params.delete('startDate');
        params.delete('endDate');
        params.delete('preset');
    }

    const newURL = params.toString() ? `${window.location.pathname}?${params.toString()}` : window.location.pathname;
    window.history.pushState({}, '', newURL);
}

function calculatePresetDates(preset) {
    const today = new Date();
    const year = today.getFullYear();
    const month = today.getMonth();

    switch (preset) {
        case 'lastMonth':
            // Previous full month
            const lastMonthDate = new Date(year, month - 1, 1);
            const startDate = formatDateForInput(lastMonthDate);
            const endDate = formatDateForInput(new Date(year, month, 0)); // Last day of last month
            return { startDate, endDate };

        case 'last3Months':
            return {
                startDate: formatDateForInput(new Date(year, month - 3, 1)),
                endDate: formatDateForInput(today)
            };

        case 'last6Months':
            return {
                startDate: formatDateForInput(new Date(year, month - 6, 1)),
                endDate: formatDateForInput(today)
            };

        case 'lastYear':
            return {
                startDate: formatDateForInput(new Date(year - 1, month, 1)),
                endDate: formatDateForInput(today)
            };

        case 'ytd':
            // Year-to-date (calendar year)
            return {
                startDate: `${year}-01-01`,
                endDate: formatDateForInput(today)
            };

        case 'all':
        default:
            return { startDate: '', endDate: '' };
    }
}

function formatDateForInput(date) {
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    return `${year}-${month}-${day}`;
}

function getFilterQueryString() {
    const params = getURLParams();

    if (params.preset && params.preset !== 'all') {
        const dates = calculatePresetDates(params.preset);
        if (dates.startDate && dates.endDate) {
            return `startDate=${dates.startDate}&endDate=${dates.endDate}`;
        }
    } else if (params.startDate && params.endDate) {
        return `startDate=${params.startDate}&endDate=${params.endDate}`;
    }

    return '';
}

function updateFilterBanner() {
    const params = getURLParams();
    const banner = document.getElementById('filter-banner');
    const bannerText = document.getElementById('filter-banner-text');

    if (params.preset && params.preset !== 'all') {
        const dates = calculatePresetDates(params.preset);
        banner.style.display = 'flex';
        bannerText.textContent = `Filtered: ${dates.startDate} to ${dates.endDate}`;
    } else if (params.startDate && params.endDate) {
        banner.style.display = 'flex';
        bannerText.textContent = `Filtered: ${params.startDate} to ${params.endDate}`;
    } else {
        banner.style.display = 'none';
    }
}

function updateActiveFilterButton() {
    const params = getURLParams();
    document.querySelectorAll('.filter-btn[data-preset]').forEach(btn => {
        btn.classList.remove('active');
    });

    const activePreset = params.preset || 'all';
    const activeBtn = document.querySelector(`.filter-btn[data-preset="${activePreset}"]`);
    if (activeBtn) {
        activeBtn.classList.add('active');
    }
}

// Show empty state message for a chart container
function showEmptyState(containerId, message = 'No Activity') {
    const container = document.getElementById(containerId);
    if (!container) return;

    const canvas = container.querySelector('canvas');
    if (canvas) {
        canvas.style.display = 'none';
    }

    let emptyMsg = container.querySelector('.empty-state-message');
    if (!emptyMsg) {
        emptyMsg = document.createElement('div');
        emptyMsg.className = 'empty-state-message';
        emptyMsg.style.cssText = `
            display: flex;
            align-items: center;
            justify-content: center;
            height: ${container.clientHeight || 300}px;
            color: #7f8c8d;
            font-size: 16px;
            font-weight: 500;
            text-align: center;
        `;
        container.appendChild(emptyMsg);
    }
    emptyMsg.textContent = message;
    emptyMsg.style.display = 'flex';
}

// Hide empty state message for a chart container
function hideEmptyState(containerId) {
    const container = document.getElementById(containerId);
    if (!container) return;

    const canvas = container.querySelector('canvas');
    if (canvas) {
        canvas.style.display = '';
    }

    const emptyMsg = container.querySelector('.empty-state-message');
    if (emptyMsg) {
        emptyMsg.style.display = 'none';
    }
}

// Store settings globally for color mapping
let appSettings = null;
let budgetCharts = [];
let incomeBreakdownChart = null;
let categorySpendingChart = null;
let incomeExpensesChart = null;
let netWorthChart = null;
let categoryTrendsChart = null;
let yearOverYearChart = null;
let savingsRateChart = null;

// Load settings and build category-to-color mapping
async function loadSettings() {
    try {
        const response = await fetch('/api/settings');
        const data = await response.json();
        appSettings = data;
    } catch (error) {
        console.error('Error loading settings:', error);
        appSettings = null;
    }
}

// Convert hex to rgba string with fallback
function rgbaFromHex(hex, alpha) {
    const rgb = hexToRgb(hex);
    if (!rgb) return `rgba(52,152,219,${alpha})`;
    return `rgba(${rgb.r}, ${rgb.g}, ${rgb.b}, ${alpha})`;
}

// Convert hex color to RGB
function hexToRgb(hex) {
    const result = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(hex);
    return result ? {
        r: parseInt(result[1], 16),
        g: parseInt(result[2], 16),
        b: parseInt(result[3], 16)
    } : null;
}

// Adjust color brightness
function adjustBrightness(hex, factor) {
    const rgb = hexToRgb(hex);
    if (!rgb) return hex;

    const r = Math.min(255, Math.max(0, Math.round(rgb.r * factor)));
    const g = Math.min(255, Math.max(0, Math.round(rgb.g * factor)));
    const b = Math.min(255, Math.max(0, Math.round(rgb.b * factor)));

    return '#' + [r, g, b].map(x => {
        const hex = x.toString(16);
        return hex.length === 1 ? '0' + hex : hex;
    }).join('');
}

// Get tier info for a category
function getTierInfo(category) {
    if (!appSettings || !appSettings.tiers) {
        return { color: '#95a5a6', tierCategories: [], position: -1 };
    }

    for (const tier of appSettings.tiers) {
        if (tier.categories && tier.categories.includes(category)) {
            const position = tier.categories.indexOf(category);
            return {
                color: tier.color || '#95a5a6',
                tierCategories: tier.categories,
                position: position
            };
        }
    }

    return { color: '#95a5a6', tierCategories: [], position: -1 };
}

// Get tier color by tier name
function getTierColorByName(tierName) {
    if (!appSettings || !appSettings.tiers) {
        return '#95a5a6';
    }

    for (const tier of appSettings.tiers) {
        if (tier.name === tierName) {
            return tier.color || '#95a5a6';
        }
    }

    return '#95a5a6';
}

// Get color with brightness variation for charts
function getCategoryColor(category) {
    const tierInfo = getTierInfo(category);
    if (tierInfo.position === -1) {
        return tierInfo.color;
    }

    // Create brightness variations within the tier
    const totalInTier = tierInfo.tierCategories.length;
    const brightnessFactor = 0.6 + (tierInfo.position / (totalInTier + 2)) * 0.8;
    return adjustBrightness(tierInfo.color, brightnessFactor);
}

// Refresh status helper
function setRefreshStatus(message) {
    const el = document.getElementById('refreshStatus');
    if (el) {
        el.textContent = message || '';
    }
}

// Manually refresh cached data and reload dashboard
async function refreshDashboardData() {
    const btn = document.getElementById('refreshButton');
    try {
        if (btn) {
            btn.disabled = true;
            btn.textContent = 'Refreshing...';
        }
        setRefreshStatus('Refreshing data...');

        const resp = await fetch('/api/cache/refresh', { method: 'POST' });
        // Even if 202 (already in progress), proceed to reload once request accepted
        if (!resp.ok && resp.status !== 202) {
            throw new Error(`Refresh failed: ${resp.status}`);
        }

        // Reload settings and all dashboard data after refresh
        await loadSettings();
        await Promise.all([
            loadSummary(),
            loadBudgetHistory(),
            loadIncomeCategories(),
            loadAccounts(),
            loadTransactions(),
            loadIncomeExpensesChart(),
            loadCategorySpendingChart(),
            loadIncomeBreakdownChart(),
            loadNetWorthChart(),
            loadCategoryTrendsChart(),
            loadYearOverYearChart()
        ]);

        setRefreshStatus('Refresh complete');
    } catch (err) {
        console.error('Error refreshing dashboard:', err);
        setRefreshStatus('Refresh failed');
    } finally {
        if (btn) {
            btn.disabled = false;
            btn.textContent = 'Refresh';
        }
        // Clear status after a short delay
        setTimeout(() => setRefreshStatus(''), 2000);
    }
}

// Load summary data
async function loadSummary() {
    try {
        const filterParams = getFilterQueryString();
        const url = filterParams ? `/api/summary?${filterParams}` : '/api/summary';
        const response = await fetch(url);
        const data = await response.json();

        document.getElementById('netWorth').textContent = formatCurrency(data.netWorth || 0);
        document.getElementById('totalAssets').textContent = formatCurrency(data.totalAssets || 0);
        document.getElementById('totalLiabilities').textContent = formatCurrency(data.totalLiabilities || 0);
    } catch (error) {
        console.error('Error loading summary:', error);
    }
}

// Load accounts
async function loadAccounts() {
    try {
        const filterParams = getFilterQueryString();
        const url = filterParams ? `/api/accounts?${filterParams}` : '/api/accounts';
        const response = await fetch(url);
        const accounts = await response.json();

        const container = document.getElementById('accounts');
        if (!accounts || accounts.length === 0) {
            container.innerHTML = '<p>No accounts found</p>';
            return;
        }

        // Filter and format accounts
        const filteredAccounts = accounts
            .filter(account => !account.aname.includes(':transfer'))
            .map(account => {
                let displayName = account.aname;
                let isLiability = false;

                // Check if it's a liability account
                if (displayName.toLowerCase().startsWith('liabilities:')) {
                    isLiability = true;
                    displayName = displayName.replace(/^liabilities:/i, '');
                } else {
                    // Strip leading "assets:" prefix
                    displayName = displayName.replace(/^assets:/i, '');
                }

                // Strip trailing ":current" suffix
                displayName = displayName.replace(/:current$/i, '');

                // For liabilities, flip the sign to show positive amounts
                let displayBalance = account.aebalance || 0;
                if (isLiability && displayBalance !== 0) {
                    displayBalance = -displayBalance;
                }

                return {
                    ...account,
                    displayName: displayName,
                    displayBalance: displayBalance,
                    isLiability: isLiability
                };
            });

        container.innerHTML = filteredAccounts.map(account => `
            <div class="account-item ${account.isLiability ? 'liability-account' : ''}" 
                 style="cursor: pointer;" 
                 onclick="navigateToDetail('account', '${escapeHtml(account.aname)}')">
                <div class="account-name">${escapeHtml(account.displayName)}</div>
                <div class="account-balance">${formatCurrency(account.displayBalance)}</div>
            </div>
        `).join('');
    } catch (error) {
        console.error('Error loading accounts:', error);
        document.getElementById('accounts').innerHTML = '<p>Error loading accounts</p>';
    }
}

// Destroy any existing budget charts before re-rendering
function destroyBudgetCharts() {
    budgetCharts.forEach(chart => chart.destroy());
    budgetCharts = [];
}

// Load budget history and render per-category line charts with multi-year support
async function loadBudgetHistory() {
    try {
        const filterParams = getFilterQueryString();
        const url = filterParams ? `/api/budget/history?${filterParams}` : '/api/budget/history';
        const response = await fetch(url);
        const items = await response.json();

        const container = document.getElementById('budget-charts');
        if (!container) return;

        destroyBudgetCharts();

        if (!items || items.length === 0) {
            container.classList.remove('budget-charts-grid');
            container.innerHTML = '<p class="budget-empty">No budget history available (need at least 2 months of history)</p>';
            return;
        }

        container.classList.add('budget-charts-grid');
        container.innerHTML = '';

        // Render a compact line chart per category with multi-year support
        items.forEach(item => {
            const card = document.createElement('div');
            card.className = 'budget-chart-card';

            const header = document.createElement('div');
            header.className = 'budget-chart-header';
            const avgExcl = item.averageExcludingExtremes || item.average || 0;
            header.innerHTML = `
                <div>
                    <div class="budget-chart-title">${escapeHtml(item.category)}</div>
                    <div class="budget-chart-meta">Avg ${formatCurrency(item.average || 0)} | Excl. extremes ${formatCurrency(avgExcl)}</div>
                </div>
            `;

            const canvas = document.createElement('canvas');
            canvas.className = 'budget-chart-canvas';
            const canvasId = `budget-chart-${(item.category || 'cat').toLowerCase().replace(/[^a-z0-9]+/g, '-')}-${Math.random().toString(36).slice(2, 6)}`;
            canvas.id = canvasId;
            canvas.height = 200;
            canvas.style.height = '200px';
            canvas.style.width = '100%';

            card.appendChild(header);
            card.appendChild(canvas);
            container.appendChild(card);

            // Group months by year
            const yearGroups = {};
            if (item.months) {
                item.months.forEach(m => {
                    const year = m.year || 'Unknown';
                    if (!yearGroups[year]) {
                        yearGroups[year] = [];
                    }
                    yearGroups[year].push(m);
                });
            }

            // Get all unique month names (01-12) for x-axis labels
            const monthNumbers = new Set();
            if (item.months) {
                item.months.forEach(m => {
                    // Extract month number from YYYY-MM format
                    if (m.month && m.month.length >= 7) {
                        monthNumbers.add(m.month.substring(5, 7));
                    }
                });
            }
            const monthLabels = Array.from(monthNumbers).sort();

            // Create dataset for each year
            const datasets = [];
            const years = Object.keys(yearGroups).sort();

            // Color palette for different years
            const yearColors = [
                '#3498db', // Blue
                '#e74c3c', // Red
                '#2ecc71', // Green
                '#f39c12', // Orange
                '#9b59b6', // Purple
                '#1abc9c', // Turquoise
                '#34495e', // Dark gray
            ];

            years.forEach((year, index) => {
                const yearData = yearGroups[year];
                const monthMap = {};
                yearData.forEach(m => {
                    if (m.month && m.month.length >= 7) {
                        const monthNum = m.month.substring(5, 7);
                        monthMap[monthNum] = m;
                    }
                });

                const dataPoints = monthLabels.map(monthNum => {
                    const entry = monthMap[monthNum];
                    return entry ? entry.amount : null;
                });

                const color = yearColors[index % yearColors.length];
                datasets.push({
                    label: year,
                    data: dataPoints,
                    borderColor: color,
                    backgroundColor: rgbaFromHex(color, 0.1),
                    tension: 0.25,
                    fill: false,
                    pointRadius: 3,
                    pointHoverRadius: 5,
                    spanGaps: true
                });
            });

            // Add average lines
            datasets.push({
                label: 'Average',
                data: monthLabels.map(() => item.average || 0),
                borderColor: '#7f8c8d',
                borderWidth: 2,
                borderDash: [6, 4],
                pointRadius: 0,
                fill: false,
                tension: 0
            });

            datasets.push({
                label: 'Avg (excl. extremes)',
                data: monthLabels.map(() => avgExcl),
                borderColor: '#95a5a6',
                borderWidth: 1.5,
                borderDash: [3, 3],
                pointRadius: 0,
                fill: false,
                tension: 0
            });

            // Convert month numbers to readable labels (Jan, Feb, etc.)
            const monthNames = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
            const xLabels = monthLabels.map(m => monthNames[parseInt(m) - 1] || m);

            // Cap y-axis at 2x the average to avoid extreme outliers
            const yAxisMax = (item.average || 0) * 2;

            const ctx = canvas.getContext('2d');
            const chart = new Chart(ctx, {
                type: 'line',
                data: {
                    labels: xLabels,
                    datasets: datasets
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    onClick: () => {
                        navigateToDetail('category', item.category);
                    },
                    scales: {
                        x: {
                            ticks: { autoSkip: true, maxTicksLimit: 12 }
                        },
                        y: {
                            beginAtZero: true,
                            max: yAxisMax > 0 ? yAxisMax : undefined,
                            ticks: {
                                callback: (value) => formatCurrency(value)
                            }
                        }
                    },
                    plugins: {
                        legend: {
                            display: true,
                            position: 'top',
                            labels: {
                                usePointStyle: true,
                                padding: 10,
                                font: { size: 11 }
                            }
                        },
                        tooltip: {
                            callbacks: {
                                label: (ctx) => {
                                    return `${ctx.dataset.label}: ${formatCurrency(ctx.parsed.y || 0)}`;
                                }
                            }
                        }
                    }
                }
            });

            // Add cursor pointer style to the card
            card.style.cursor = 'pointer';

            budgetCharts.push(chart);
        });
    } catch (error) {
        console.error('Error loading budget history:', error);
        const container = document.getElementById('budget-charts');
        if (container) {
            container.classList.remove('budget-charts-grid');
            container.innerHTML = '<p class="budget-empty">Error loading budget history</p>';
        }
    }
}

// Load income categories and render as a single clickable 'All Income' card
async function loadIncomeCategories() {
    try {
        const filterParams = getFilterQueryString();
        const url = filterParams ? `/api/income-breakdown?${filterParams}` : '/api/income-breakdown';
        const response = await fetch(url);
        const income = await response.json();

        const container = document.getElementById('income-categories');
        if (!container) return;

        if (!income || income.length === 0) {
            container.classList.remove('budget-charts-grid');
            container.innerHTML = '<p class="budget-empty">No income data available</p>';
            return;
        }

        container.classList.add('budget-charts-grid');
        container.innerHTML = '';

        // Calculate total income across all categories
        const totalIncome = income.reduce((sum, item) => sum + item.amount, 0);
        const categoryCount = income.length;

        // Render a single 'All Income' card
        const card = document.createElement('div');
        card.className = 'budget-chart-card';
        card.style.cursor = 'pointer';
        card.onclick = () => navigateToDetail('income-all', 'all');

        const header = document.createElement('div');
        header.className = 'budget-chart-header';
        header.innerHTML = `
            <div>
                <div class="budget-chart-title">All Income</div>
                <div class="budget-chart-meta">${categoryCount} ${categoryCount === 1 ? 'category' : 'categories'} | Total: ${formatCurrency(totalIncome)}</div>
            </div>
        `;

        card.appendChild(header);
        container.appendChild(card);
    } catch (error) {
        console.error('Error loading income categories:', error);
        const container = document.getElementById('income-categories');
        if (container) {
            container.classList.remove('budget-charts-grid');
            container.innerHTML = '<p class="budget-empty">Error loading income categories</p>';
        }
    }
}

// Extract category from account name
function getCategory(accountName) {
    const parts = accountName.split(':');
    return parts[0]; // e.g., "Assets", "Expenses", "Liabilities", "Income"
}

// Extract account from account name
function getAccount(accountName) {
    const parts = accountName.split(':');
    return parts.slice(1).join(':'); // e.g., "Checking" or "Food:Groceries"
}

// Load transactions
// Pagination state
let allTransactions = [];
let currentPage = 1;
const transactionsPerPage = 20;

async function loadTransactions() {
    try {
        const filterParams = getFilterQueryString();
        const url = filterParams ? `/api/transactions?${filterParams}` : '/api/transactions';
        const response = await fetch(url);
        allTransactions = await response.json();
        currentPage = 1;
        renderTransactionsPage();
    } catch (error) {
        console.error('Error loading transactions:', error);
        document.getElementById('transactions').innerHTML = '<p>Error loading transactions</p>';
    }
}

function renderTransactionsPage() {
    const container = document.getElementById('transactions');
    const pagination = document.getElementById('transactions-pagination');

    if (!allTransactions || allTransactions.length === 0) {
        container.innerHTML = '<p>No transactions found</p>';
        if (pagination) pagination.style.display = 'none';
        return;
    }

    const totalPages = Math.ceil(allTransactions.length / transactionsPerPage);
    const startIdx = (currentPage - 1) * transactionsPerPage;
    const endIdx = Math.min(startIdx + transactionsPerPage, allTransactions.length);
    const pageTransactions = allTransactions.slice(startIdx, endIdx);

    let html = `
        <table class="transactions-table">
            <thead>
                <tr>
                    <th>Date</th>
                    <th>Description</th>
                    <th>Account</th>
                    <th>Category</th>
                    <th class="amount-col">Amount</th>
                </tr>
            </thead>
            <tbody>
    `;

    html += pageTransactions.map(tx => {
        // Extract amount from the first posting
        let amount = 0;
        if (tx.tpostings && tx.tpostings.length > 0 && tx.tpostings[0].pamount && tx.tpostings[0].pamount.length > 0) {
            const quantity = tx.tpostings[0].pamount[0].aquantity;
            amount = quantity.decimalMantissa / Math.pow(10, quantity.decimalPlaces);
        }

        // Find the asset/liability account and the income/expense category
        let account = '';
        let category = '';

        if (tx.tpostings) {
            // Find first Assets or Liabilities posting
            const assetLiability = tx.tpostings.find(p =>
                p.paccount.startsWith('assets:') || p.paccount.startsWith('liabilities:')
            );
            if (assetLiability) {
                account = getAccount(assetLiability.paccount);
            }

            // Find first Expenses or Income posting
            const expenseIncome = tx.tpostings.find(p =>
                p.paccount.startsWith('expenses:') || p.paccount.startsWith('income:')
            );
            if (expenseIncome) {
                category = getAccount(expenseIncome.paccount);
            }
        }

        return `
                <tr>
                    <td class="date-col">${formatDate(tx.tdate)}</td>
                    <td class="desc-col">${escapeHtml(tx.tdescription)}</td>
                    <td class="account-col">${escapeHtml(account)}</td>
                    <td class="category-col">${escapeHtml(category)}</td>
                    <td class="amount-col">${formatCurrency(amount)}</td>
                </tr>
        `;
    }).join('');

    html += `
            </tbody>
        </table>
    `;

    container.innerHTML = html;

    // Update pagination controls
    if (pagination) {
        if (totalPages > 1) {
            pagination.style.display = 'flex';
            const firstBtn = document.getElementById('first-page');
            const prevBtn = document.getElementById('prev-page');
            const nextBtn = document.getElementById('next-page');
            const lastBtn = document.getElementById('last-page');
            const pageInfo = document.getElementById('page-info');
            const pageInput = document.getElementById('page-input');
            const totalPagesSpan = document.getElementById('total-pages');

            if (firstBtn) firstBtn.disabled = currentPage === 1;
            if (prevBtn) prevBtn.disabled = currentPage === 1;
            if (nextBtn) nextBtn.disabled = currentPage === totalPages;
            if (lastBtn) lastBtn.disabled = currentPage === totalPages;

            if (pageInput) {
                pageInput.value = currentPage;
                pageInput.max = totalPages;
            }

            if (totalPagesSpan) {
                totalPagesSpan.textContent = totalPages;
            }

            if (pageInfo) {
                pageInfo.textContent = `(${allTransactions.length} transactions)`;
            }
        } else {
            pagination.style.display = 'none';
        }
    }
}

function goToFirstPage() {
    currentPage = 1;
    renderTransactionsPage();
}

function goToPreviousPage() {
    if (currentPage > 1) {
        currentPage--;
        renderTransactionsPage();
    }
}

function goToNextPage() {
    const totalPages = Math.ceil(allTransactions.length / transactionsPerPage);
    if (currentPage < totalPages) {
        currentPage++;
        renderTransactionsPage();
    }
}

function goToLastPage() {
    const totalPages = Math.ceil(allTransactions.length / transactionsPerPage);
    currentPage = totalPages;
    renderTransactionsPage();
}

function goToPage(pageNum) {
    const totalPages = Math.ceil(allTransactions.length / transactionsPerPage);
    const page = parseInt(pageNum);
    if (page >= 1 && page <= totalPages) {
        currentPage = page;
        renderTransactionsPage();
    }
}

// Format date
function formatDate(dateString) {
    if (!dateString) return '';
    const date = new Date(dateString);
    return date.toLocaleDateString('en-US', {
        month: 'short',
        day: 'numeric',
        year: 'numeric'
    });
}

// Escape HTML
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Load and render savings rate chart
async function loadSavingsRateChart() {
    try {
        const filterParams = getFilterQueryString();
        const url = filterParams ? `/api/monthly-metrics?${filterParams}` : '/api/monthly-metrics';
        const response = await fetch(url);
        const metrics = await response.json();

        if (!metrics || metrics.length === 0) {
            return;
        }

        const ctx = document.getElementById('savings-rate-chart');
        if (!ctx) return;

        // Calculate savings amounts and averages
        const savingsAmounts = metrics.map(m => m.income - m.expenses);

        // Calculate full average
        const sum = savingsAmounts.reduce((a, b) => a + b, 0);
        const average = sum / savingsAmounts.length;

        // Calculate average excluding extremes (> 2x average)
        const filteredAmounts = savingsAmounts.filter(v => v <= average * 2);
        const avgExcludingExtremes = filteredAmounts.length > 0
            ? filteredAmounts.reduce((a, b) => a + b, 0) / filteredAmounts.length
            : average;

        // Group by year
        const yearGroups = {};
        metrics.forEach(m => {
            const year = m.month.substring(0, 4);
            if (!yearGroups[year]) {
                yearGroups[year] = [];
            }
            yearGroups[year].push(m);
        });

        // Get unique month numbers
        const monthNumbers = new Set();
        metrics.forEach(m => {
            if (m.month && m.month.length >= 7) {
                monthNumbers.add(m.month.substring(5, 7));
            }
        });
        const monthLabels = Array.from(monthNumbers).sort();

        // Color palette for different years
        const yearColors = [
            '#3498db', // Blue
            '#e74c3c', // Red
            '#2ecc71', // Green
            '#f39c12', // Orange
            '#9b59b6', // Purple
            '#1abc9c', // Turquoise
            '#34495e', // Dark gray
        ];

        const datasets = [];
        const years = Object.keys(yearGroups).sort();

        years.forEach((year, index) => {
            const yearData = yearGroups[year];
            const monthMap = {};
            yearData.forEach(m => {
                if (m.month && m.month.length >= 7) {
                    const monthNum = m.month.substring(5, 7);
                    monthMap[monthNum] = m;
                }
            });

            const dataPoints = monthLabels.map(monthNum => {
                const entry = monthMap[monthNum];
                return entry ? (entry.income - entry.expenses) : null;
            });

            const color = yearColors[index % yearColors.length];
            datasets.push({
                label: year,
                data: dataPoints,
                borderColor: color,
                backgroundColor: rgbaFromHex(color, 0.1),
                tension: 0.25,
                fill: false,
                pointRadius: 3,
                pointHoverRadius: 5,
                spanGaps: true
            });
        });

        // Add average lines
        datasets.push({
            label: 'Average',
            data: monthLabels.map(() => average),
            borderColor: '#7f8c8d',
            borderWidth: 2,
            borderDash: [6, 4],
            pointRadius: 0,
            fill: false,
            tension: 0
        });

        datasets.push({
            label: 'Avg (excl. extremes)',
            data: monthLabels.map(() => avgExcludingExtremes),
            borderColor: '#95a5a6',
            borderWidth: 1.5,
            borderDash: [3, 3],
            pointRadius: 0,
            fill: false,
            tension: 0
        });

        // Convert month numbers to readable labels
        const monthNames = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
        const xLabels = monthLabels.map(m => monthNames[parseInt(m) - 1] || m);

        // Update the meta information
        const metaEl = document.getElementById('savings-chart-meta');
        if (metaEl) {
            metaEl.innerHTML = `Avg ${formatCurrency(average)} | Excl. extremes ${formatCurrency(avgExcludingExtremes)}`;
        }

        new Chart(ctx, {
            type: 'line',
            data: {
                labels: xLabels,
                datasets: datasets
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    x: {
                        ticks: { autoSkip: true, maxTicksLimit: 12 }
                    },
                    y: {
                        beginAtZero: true,
                        ticks: {
                            callback: (value) => formatCurrency(value)
                        }
                    }
                },
                plugins: {
                    legend: {
                        display: true,
                        position: 'top',
                        labels: {
                            usePointStyle: true,
                            padding: 10,
                            font: { size: 11 }
                        }
                    },
                    tooltip: {
                        callbacks: {
                            label: (ctx) => {
                                return `${ctx.dataset.label}: ${formatCurrency(ctx.parsed.y || 0)}`;
                            }
                        }
                    }
                }
            }
        });
    } catch (error) {
        console.error('Error loading savings rate chart:', error);
    }
}

// Load and render income vs expenses chart
async function loadIncomeExpensesChart() {
    try {
        const filterParams = getFilterQueryString();
        const url = filterParams ? `/api/monthly-metrics?${filterParams}` : '/api/monthly-metrics';
        const response = await fetch(url);
        const metrics = await response.json();

        if (!metrics || metrics.length === 0) {
            showEmptyState('incomeExpensesChart', 'No Activity');
            return;
        }

        hideEmptyState('incomeExpensesChart');
        const ctx = document.getElementById('incomeExpensesChart');
        if (!ctx) return;

        const labels = metrics.map(m => m.month);
        const incomeData = metrics.map(m => m.income);
        const expensesData = metrics.map(m => m.expenses);
        const savingsData = metrics.map(m => m.income - m.expenses);

        new Chart(ctx, {
            type: 'bar',
            data: {
                labels: labels,
                datasets: [
                    {
                        type: 'bar',
                        label: 'Savings',
                        data: savingsData,
                        backgroundColor: '#27ae60',
                        borderColor: '#229954',
                        borderWidth: 1,
                        yAxisID: 'y',
                        order: 3
                    },
                    {
                        type: 'line',
                        label: 'Income',
                        data: incomeData,
                        borderColor: '#2ecc71',
                        backgroundColor: 'rgba(46, 204, 113, 0.1)',
                        borderWidth: 2,
                        fill: false,
                        tension: 0.3,
                        yAxisID: 'y',
                        order: 1
                    },
                    {
                        type: 'line',
                        label: 'Expenses',
                        data: expensesData,
                        borderColor: '#e74c3c',
                        backgroundColor: 'rgba(231, 76, 60, 0.1)',
                        borderWidth: 2,
                        fill: false,
                        tension: 0.3,
                        yAxisID: 'y',
                        order: 2
                    }
                ]
            },
            options: {
                responsive: true,
                maintainAspectRatio: true,
                interaction: {
                    mode: 'index',
                    intersect: false
                },
                plugins: {
                    legend: {
                        position: 'top',
                    }
                },
                scales: {
                    y: {
                        beginAtZero: true,
                        ticks: {
                            callback: function (value) {
                                return '$' + value.toLocaleString();
                            }
                        }
                    }
                },
                onClick: async (event, elements) => {
                    if (elements.length > 0) {
                        const datasetIndex = elements[0].datasetIndex;
                        // datasetIndex 1 is the Income line
                        if (datasetIndex === 1) {
                            // Fetch income breakdown to find main income category
                            try {
                                const response = await fetch('/api/income-breakdown');
                                const breakdown = await response.json();
                                if (breakdown && breakdown.length > 0) {
                                    // Navigate to the first/largest income category
                                    navigateToIncomeDetail(breakdown[0].category);
                                }
                            } catch (error) {
                                console.error('Error fetching income breakdown:', error);
                            }
                        }
                    }
                }
            }
        });
    } catch (error) {
        console.error('Error loading income/expenses chart:', error);
    }
}

// Load and render spending by category chart
async function loadCategorySpendingChart() {
    try {
        const filterParams = getFilterQueryString();
        const url = filterParams ? `/api/category-spending?${filterParams}` : '/api/category-spending';
        const response = await fetch(url);
        const spending = await response.json();

        if (!spending || spending.length === 0) {
            showEmptyState('categorySpendingChart', 'No Activity');
            return;
        }

        hideEmptyState('categorySpendingChart');
        const ctx = document.getElementById('categorySpendingChart');
        if (!ctx) return;

        // Group by category and sum all months
        const categoryTotals = {};
        spending.forEach(item => {
            if (!categoryTotals[item.category]) {
                categoryTotals[item.category] = 0;
            }
            categoryTotals[item.category] += item.amount;
        });

        // Sort categories by total spending (descending)
        const sortedCategories = Object.entries(categoryTotals)
            .sort((a, b) => b[1] - a[1]);

        // Take top 10 and aggregate the rest as "Other"
        const TOP_N = 10;
        let labels = [];
        let data = [];

        if (sortedCategories.length <= TOP_N) {
            // If we have 10 or fewer categories, show them all
            labels = sortedCategories.map(([cat, _]) => cat);
            data = sortedCategories.map(([_, amount]) => amount);
        } else {
            // Show top 10 + "Other"
            const top10 = sortedCategories.slice(0, TOP_N);
            const rest = sortedCategories.slice(TOP_N);

            labels = top10.map(([cat, _]) => cat);
            data = top10.map(([_, amount]) => amount);

            // Calculate "Other" total
            const otherTotal = rest.reduce((sum, [_, amount]) => sum + amount, 0);
            if (otherTotal > 0) {
                labels.push('Other');
                data.push(otherTotal);
            }
        }

        // Get tier colors for each category with brightness variation
        const colors = labels.map(label => {
            if (label === 'Other') {
                return '#95a5a6'; // Gray for "Other"
            }
            return getCategoryColor(label);
        });

        new Chart(ctx, {
            type: 'doughnut',
            data: {
                labels: labels,
                datasets: [{
                    data: data,
                    backgroundColor: colors,
                    borderColor: '#ffffff',
                    borderWidth: 2
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: true,
                onClick: (event, elements) => {
                    if (elements.length > 0) {
                        const index = elements[0].index;
                        const category = labels[index];
                        if (category !== 'Other') {
                            navigateToDetail('category', category);
                        }
                    }
                },
                plugins: {
                    legend: {
                        position: 'bottom',
                    },
                    tooltip: {
                        callbacks: {
                            label: function (context) {
                                return '$' + context.parsed.toFixed(2);
                            }
                        }
                    }
                }
            }
        });
    } catch (error) {
        console.error('Error loading category spending chart:', error);
    }
}

// Initialize dashboard
document.addEventListener('DOMContentLoaded', () => {
    const refreshBtn = document.getElementById('refreshButton');
    if (refreshBtn) {
        refreshBtn.addEventListener('click', refreshDashboardData);
    }

    // Add filter event listeners
    document.querySelectorAll('.filter-btn[data-preset]').forEach(btn => {
        btn.addEventListener('click', () => {
            const preset = btn.dataset.preset;
            const dates = calculatePresetDates(preset);
            setURLParams(dates.startDate, dates.endDate, preset);
            updateFilterBanner();
            updateActiveFilterButton();
            currentPage = 1; // Reset pagination
            refreshAllData();
        });
    });

    const applyCustomBtn = document.getElementById('apply-custom-filter');
    if (applyCustomBtn) {
        applyCustomBtn.addEventListener('click', () => {
            const startDate = document.getElementById('filter-start').value;
            const endDate = document.getElementById('filter-end').value;
            if (startDate && endDate) {
                setURLParams(startDate, endDate, null);
                updateFilterBanner();
                updateActiveFilterButton();
                currentPage = 1; // Reset pagination
                refreshAllData();
            }
        });
    }

    const clearFilterBtn = document.getElementById('clear-filter-btn');
    if (clearFilterBtn) {
        clearFilterBtn.addEventListener('click', () => {
            setURLParams('', '', 'all');
            document.getElementById('filter-start').value = '';
            document.getElementById('filter-end').value = '';
            updateFilterBanner();
            updateActiveFilterButton();
            currentPage = 1; // Reset pagination
            refreshAllData();
        });
    }

    // Initialize filter UI from URL
    updateFilterBanner();
    updateActiveFilterButton();

    // Add pagination event listeners
    const firstBtn = document.getElementById('first-page');
    const prevBtn = document.getElementById('prev-page');
    const nextBtn = document.getElementById('next-page');
    const lastBtn = document.getElementById('last-page');
    const pageInput = document.getElementById('page-input');

    if (firstBtn) firstBtn.addEventListener('click', goToFirstPage);
    if (prevBtn) prevBtn.addEventListener('click', goToPreviousPage);
    if (nextBtn) nextBtn.addEventListener('click', goToNextPage);
    if (lastBtn) lastBtn.addEventListener('click', goToLastPage);

    if (pageInput) {
        pageInput.addEventListener('change', (e) => goToPage(e.target.value));
        pageInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                goToPage(e.target.value);
            }
        });
    }

    // Check if this is a detail page
    const urlParams = new URLSearchParams(window.location.search);
    if (urlParams.get('page') === 'detail') {
        loadDetailPage();
    } else {
        loadSettings().then(() => {
            loadSummary();
            loadBudgetHistory();
            loadIncomeCategories();
            loadSavingsRateChart();
            loadAccounts();
            loadTransactions();
            loadIncomeExpensesChart();
            loadCategorySpendingChart();
            loadIncomeBreakdownChart();
            loadNetWorthChart();
            loadCategoryTrendsChart();
            loadYearOverYearChart();
        });
    }
});

function refreshAllData() {
    loadSummary();
    loadBudgetHistory();
    loadIncomeCategories();
    loadSavingsRateChart();
    loadAccounts();
    loadTransactions();
    loadIncomeExpensesChart();
    loadCategorySpendingChart();
    loadIncomeBreakdownChart();
    loadNetWorthChart();
    loadCategoryTrendsChart();
    loadYearOverYearChart();
}


// Convert YYYY-MM to Date for chart display
function parseMonthToDate(monthStr) {
    const [year, month] = monthStr.split('-');
    return new Date(year, parseInt(month) - 1, 1);
}

// Load and render income breakdown chart
async function loadIncomeBreakdownChart() {
    try {
        // Destroy previous chart if it exists
        if (incomeBreakdownChart) {
            incomeBreakdownChart.destroy();
            incomeBreakdownChart = null;
        }

        const filterParams = getFilterQueryString();
        const url = filterParams ? `/api/income-breakdown?${filterParams}` : '/api/income-breakdown';
        const response = await fetch(url);
        const income = await response.json();

        if (!income || income.length === 0) {
            showEmptyState('incomeBreakdownChart', 'No Activity');
            return;
        }

        hideEmptyState('incomeBreakdownChart');
        const ctx = document.getElementById('incomeBreakdownChart');
        if (!ctx) return;

        const labels = income.map(item => item.category);
        const data = income.map(item => item.amount);

        // Generate distinct colors for income categories (shades of green)
        const colors = labels.map((_, index) => {
            const hue = 140 + (index * 20) % 60; // Green hues
            const lightness = 45 + (index * 5) % 20;
            return `hsl(${hue}, 60%, ${lightness}%)`;
        });

        incomeBreakdownChart = new Chart(ctx, {
            type: 'doughnut',
            data: {
                labels: labels,
                datasets: [{
                    data: data,
                    backgroundColor: colors,
                    borderColor: '#ffffff',
                    borderWidth: 2
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: true,
                onClick: (event, elements) => {
                    if (elements.length > 0) {
                        const index = elements[0].index;
                        const category = labels[index];
                        navigateToIncomeDetail(category);
                    }
                },
                plugins: {
                    legend: {
                        position: 'bottom',
                    },
                    tooltip: {
                        callbacks: {
                            label: function (context) {
                                return '$' + context.parsed.toFixed(2);
                            }
                        }
                    }
                }
            }
        });
    } catch (error) {
        console.error('Error loading income breakdown chart:', error);
    }
}

// Load and render net worth over time chart
async function loadNetWorthChart() {
    try {
        const filterParams = getFilterQueryString();
        const url = filterParams ? `/api/net-worth-over-time?${filterParams}` : '/api/net-worth-over-time';
        const response = await fetch(url);
        const data = await response.json();

        if (!data || data.length === 0) {
            showEmptyState('netWorthChart', 'No Activity');
            return;
        }

        hideEmptyState('netWorthChart');
        const ctx = document.getElementById('netWorthChart');
        if (!ctx) return;

        const chartData = data.map(point => ({
            x: new Date(point.date),
            y: point.netWorth
        }));

        new Chart(ctx, {
            type: 'line',
            data: {
                datasets: [{
                    label: 'Net Worth',
                    data: chartData,
                    borderColor: '#3498db',
                    backgroundColor: 'rgba(52, 152, 219, 0.1)',
                    borderWidth: 2,
                    fill: true,
                    tension: 0.1,
                    pointRadius: 0,
                    pointHoverRadius: 6
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: true,
                plugins: {
                    legend: {
                        position: 'top',
                    }
                },
                scales: {
                    x: {
                        type: 'time',
                        time: {
                            unit: 'day',
                            displayFormats: {
                                day: 'MMM d, yyyy'
                            }
                        }
                    },
                    y: {
                        beginAtZero: true,
                        ticks: {
                            callback: function (value) {
                                return '$' + value.toLocaleString();
                            }
                        }
                    }
                }
            }
        });
    } catch (error) {
        console.error('Error loading net worth chart:', error);
    }
}

// Load and render category trends chart
async function loadCategoryTrendsChart() {
    try {
        const filterParams = getFilterQueryString();
        const url = filterParams ? `/api/category-trends?${filterParams}` : '/api/category-trends';
        const response = await fetch(url);
        const data = await response.json();

        if (!data || data.length === 0) {
            showEmptyState('categoryTrendsChart', 'No Activity');
            return;
        }

        hideEmptyState('categoryTrendsChart');
        const ctx = document.getElementById('categoryTrendsChart');
        if (!ctx) return;

        const datasets = data.map((cat, idx) => {
            const chartData = cat.data.map(ma => ({
                x: parseMonthToDate(ma.month),
                y: ma.amount
            }));

            // For trends chart, use tier color by name since it's aggregated by tier
            const categoryColor = getTierColorByName(cat.category);

            return {
                label: cat.category,
                data: chartData,
                borderColor: categoryColor,
                backgroundColor: categoryColor,
                borderWidth: 2,
                fill: false,
                tension: 0.3
            };
        });

        new Chart(ctx, {
            type: 'line',
            data: {
                datasets: datasets
            },
            options: {
                responsive: true,
                maintainAspectRatio: true,
                onClick: (event, elements) => {
                    if (elements.length > 0) {
                        const datasetIndex = elements[0].datasetIndex;
                        const tierName = data[datasetIndex].category;
                        navigateToDetail('tier', tierName);
                    }
                },
                interaction: {
                    mode: 'index',
                    intersect: false
                },
                plugins: {
                    legend: {
                        position: 'top',
                    }
                },
                scales: {
                    x: {
                        type: 'time',
                        time: {
                            unit: 'month',
                            displayFormats: {
                                month: 'MMM yyyy'
                            }
                        }
                    },
                    y: {
                        beginAtZero: true,
                        ticks: {
                            callback: function (value) {
                                return '$' + value.toLocaleString();
                            }
                        }
                    }
                }
            }
        });
    } catch (error) {
        console.error('Error loading category trends chart:', error);
    }
}

// Load and render year-over-year comparison chart
async function loadYearOverYearChart() {
    try {
        const filterParams = getFilterQueryString();
        const url = filterParams ? `/api/year-over-year-comparison?${filterParams}` : '/api/year-over-year-comparison';
        const response = await fetch(url);
        const data = await response.json();

        if (!data || data.length === 0) {
            showEmptyState('yoyComparisonChart', 'No Activity');
            return;
        }

        hideEmptyState('yoyComparisonChart');
        const ctx = document.getElementById('yoyComparisonChart');
        if (!ctx) return;

        // Get all unique years
        const yearsSet = new Set();
        data.forEach(item => {
            Object.keys(item.years).forEach(year => {
                yearsSet.add(year);
            });
        });
        const years = Array.from(yearsSet).sort();

        // Color palette
        const colors = [
            '#3498db', '#e74c3c', '#2ecc71', '#f39c12', '#9b59b6',
            '#1abc9c', '#34495e', '#e67e22', '#95a5a6', '#16a085'
        ];

        // For year-over-year, use month (1-12) as x-axis
        const datasets = years.map((year, idx) => {
            const amounts = data.map(item => ({
                x: parseInt(item.month),
                y: item.years[year] || 0
            }));

            return {
                label: year,
                data: amounts,
                backgroundColor: colors[idx % colors.length],
                borderColor: colors[idx % colors.length],
                borderWidth: 1
            };
        });

        new Chart(ctx, {
            type: 'bar',
            data: {
                datasets: datasets
            },
            options: {
                responsive: true,
                maintainAspectRatio: true,
                scales: {
                    x: {
                        type: 'linear',
                        min: 1,
                        max: 12,
                        ticks: {
                            stepSize: 1,
                            callback: function (value) {
                                const months = ['', 'Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
                                return months[value] || '';
                            }
                        }
                    },
                    y: {
                        beginAtZero: true,
                        ticks: {
                            callback: function (value) {
                                return '$' + value.toLocaleString();
                            }
                        }
                    }
                },
                plugins: {
                    legend: {
                        position: 'top',
                    }
                }
            }
        });
    } catch (error) {
        console.error('Error loading year-over-year chart:', error);
    }
}


// ===== DETAIL PAGE FUNCTIONALITY =====

let detailTransactions = [];
let detailCurrentPage = 1;
const detailItemsPerPage = 20;

// Navigate to detail page
function navigateToDetail(type, name) {
    const params = new URLSearchParams();
    params.set('page', 'detail');
    params.set(type, encodeURIComponent(name));
    window.location.href = '/?' + params.toString();
}

// Navigate to income detail page
function navigateToIncomeDetail(incomeName) {
    navigateToDetail('income', incomeName);
}

// Load detail page based on URL parameters
async function loadDetailPage() {
    await loadSettings();

    const urlParams = new URLSearchParams(window.location.search);
    const category = urlParams.get('category');
    const tier = urlParams.get('tier');
    const account = urlParams.get('account');
    const income = urlParams.get('income');
    const incomeAll = urlParams.get('income-all');

    if (category) {
        loadCategoryDetail(category);
    } else if (tier) {
        loadTierDetail(tier);
    } else if (account) {
        loadAccountDetail(account);
    } else if (income) {
        loadIncomeDetail(income);
    } else if (incomeAll) {
        loadAllIncomeDetail();
    } else {
        document.getElementById('detail-title').textContent = 'Invalid Detail View';
        document.getElementById('breadcrumb-type').textContent = 'Error';
        document.getElementById('breadcrumb-name').textContent = 'No filter specified';
    }

    // Set up detail page pagination
    setupDetailPagination();
}

// Setup pagination for detail page
function setupDetailPagination() {
    const firstBtn = document.getElementById('detail-first-page');
    const prevBtn = document.getElementById('detail-prev-page');
    const nextBtn = document.getElementById('detail-next-page');
    const lastBtn = document.getElementById('detail-last-page');
    const pageInput = document.getElementById('detail-page-input');

    if (firstBtn) firstBtn.addEventListener('click', () => goToDetailPage(1));
    if (prevBtn) prevBtn.addEventListener('click', () => goToDetailPage(detailCurrentPage - 1));
    if (nextBtn) nextBtn.addEventListener('click', () => goToDetailPage(detailCurrentPage + 1));
    if (lastBtn) lastBtn.addEventListener('click', () => {
        const totalPages = Math.ceil(detailTransactions.length / detailItemsPerPage);
        goToDetailPage(totalPages);
    });

    if (pageInput) {
        pageInput.addEventListener('change', (e) => goToDetailPage(parseInt(e.target.value)));
        pageInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                goToDetailPage(parseInt(e.target.value));
            }
        });
    }
}

// Navigate to specific page in detail view
function goToDetailPage(page) {
    const totalPages = Math.ceil(detailTransactions.length / detailItemsPerPage);
    if (page < 1 || page > totalPages) return;

    detailCurrentPage = page;
    renderDetailTransactions();
}

// Render detail transactions with pagination
function renderDetailTransactions() {
    const container = document.getElementById('detail-transactions');
    const paginationContainer = document.getElementById('detail-transactions-pagination');

    if (!detailTransactions || detailTransactions.length === 0) {
        container.innerHTML = '<p>No transactions found.</p>';
        paginationContainer.style.display = 'none';
        return;
    }

    // Get filter type from URL
    const urlParams = new URLSearchParams(window.location.search);
    const filterCategory = urlParams.get('category');
    const filterTier = urlParams.get('tier');
    const filterAccount = urlParams.get('account');
    const filterIncome = urlParams.get('income');
    const filterIncomeAll = urlParams.get('income-all');

    const totalPages = Math.ceil(detailTransactions.length / detailItemsPerPage);
    const startIndex = (detailCurrentPage - 1) * detailItemsPerPage;
    const endIndex = Math.min(startIndex + detailItemsPerPage, detailTransactions.length);
    const pageTransactions = detailTransactions.slice(startIndex, endIndex);

    let html = '<table class="transactions-table"><thead><tr>';
    html += '<th>Date</th><th>Description</th><th>Account</th><th>Amount</th>';
    html += '</tr></thead><tbody>';

    pageTransactions.forEach(tx => {
        tx.tpostings.forEach(posting => {
            // Filter postings based on what we're viewing
            let shouldShow = false;

            if (filterCategory) {
                // Show only expense postings for this category
                if (posting.paccount.startsWith('expenses:')) {
                    const parts = posting.paccount.split(':');
                    if (parts[1] === filterCategory) {
                        shouldShow = true;
                    }
                }
            } else if (filterTier) {
                // Show only expense postings for categories in this tier
                if (posting.paccount.startsWith('expenses:')) {
                    const parts = posting.paccount.split(':');
                    const category = parts[1];
                    // Find the tier and check if category is in it
                    if (appSettings && appSettings.tiers) {
                        for (const tier of appSettings.tiers) {
                            if (tier.name === filterTier && tier.categories.includes(category)) {
                                shouldShow = true;
                                break;
                            }
                        }
                    }
                }
            } else if (filterAccount) {
                // Show the posting for this account
                if (posting.paccount === filterAccount) {
                    shouldShow = true;
                }
            } else if (filterIncome) {
                // Show only income postings for this income category
                if (posting.paccount.startsWith('income:')) {
                    const parts = posting.paccount.split(':');
                    if (parts[1] === filterIncome) {
                        shouldShow = true;
                    }
                }
            } else if (filterIncomeAll) {
                // Show all income postings
                if (posting.paccount.startsWith('income:')) {
                    shouldShow = true;
                }
            }

            if (!shouldShow) {
                return;
            }

            const amount = posting.pamount && posting.pamount.length > 0
                ? convertPostingAmount(posting.pamount[0])
                : 0;
            html += `<tr>
                <td>${tx.tdate}</td>
                <td>${tx.tdescription}</td>
                <td>${posting.paccount}</td>
                <td class="${amount < 0 ? 'negative' : 'positive'}">${formatCurrency(Math.abs(amount))}</td>
            </tr>`;
        });
    });

    html += '</tbody></table>';
    container.innerHTML = html;

    // Update pagination
    paginationContainer.style.display = 'flex';
    document.getElementById('detail-page-input').value = detailCurrentPage;
    document.getElementById('detail-total-pages').textContent = totalPages;
    document.getElementById('detail-page-info').textContent = `${startIndex + 1}-${endIndex} of ${detailTransactions.length}`;

    // Enable/disable buttons
    document.getElementById('detail-first-page').disabled = detailCurrentPage === 1;
    document.getElementById('detail-prev-page').disabled = detailCurrentPage === 1;
    document.getElementById('detail-next-page').disabled = detailCurrentPage === totalPages;
    document.getElementById('detail-last-page').disabled = detailCurrentPage === totalPages;
}

// Convert posting amount to float
function convertPostingAmount(amount) {
    if (!amount || !amount.aquantity) return 0;
    const mantissa = amount.aquantity.decimalMantissa || 0;
    const places = amount.aquantity.decimalPlaces || 0;
    const divisor = Math.pow(10, places);
    return mantissa / divisor;
}

// Load category detail
async function loadCategoryDetail(category) {
    try {
        const filterParams = getFilterQueryString();
        const separator = filterParams ? '&' : '';
        const url = `/api/detail/category?category=${encodeURIComponent(category)}${separator}${filterParams}`;
        const response = await fetch(url);
        const data = await response.json();

        // Update page title and breadcrumb
        document.getElementById('detail-title').textContent = category;
        document.getElementById('breadcrumb-type').textContent = 'Category';
        document.getElementById('breadcrumb-name').textContent = category;
        document.getElementById('transactions-title').textContent = `${category} Transactions`;

        // Store transactions
        detailTransactions = data.transactions || [];
        renderDetailTransactions();

        // Render breakdown chart
        if (data.breakdown && data.breakdown.length > 0) {
            renderBreakdownChart(data.breakdown, 'Subcategories');
        }

        // Render budget history
        if (data.budgetHistory && data.budgetHistory.length > 0) {
            renderDetailBudgetHistory(data.budgetHistory);
        }
    } catch (error) {
        console.error('Error loading category detail:', error);
    }
}

// Load tier detail
async function loadTierDetail(tier) {
    try {
        const filterParams = getFilterQueryString();
        const separator = filterParams ? '&' : '';
        const url = `/api/detail/tier?tier=${encodeURIComponent(tier)}${separator}${filterParams}`;
        const response = await fetch(url);
        const data = await response.json();

        // Update page title and breadcrumb
        document.getElementById('detail-title').textContent = tier + ' Spending';
        document.getElementById('breadcrumb-type').textContent = 'Tier';
        document.getElementById('breadcrumb-name').textContent = tier;
        document.getElementById('transactions-title').textContent = `${tier} Transactions`;

        // Store transactions
        detailTransactions = data.transactions || [];
        renderDetailTransactions();

        // Render breakdown chart (categories within tier)
        if (data.breakdown && data.breakdown.length > 0) {
            renderBreakdownChart(data.breakdown, 'Categories in ' + tier);
        }

        // Render budget history (multi-line for tier)
        if (data.budgetHistory && data.budgetHistory.length > 0) {
            renderDetailBudgetHistory(data.budgetHistory, true);
        }
    } catch (error) {
        console.error('Error loading tier detail:', error);
    }
}

// Load account detail
async function loadAccountDetail(account) {
    try {
        const filterParams = getFilterQueryString();
        const separator = filterParams ? '&' : '';
        const url = `/api/detail/account?account=${encodeURIComponent(account)}${separator}${filterParams}`;
        const response = await fetch(url);
        const data = await response.json();

        // Clean up account name for display
        let displayName = account;
        if (account.startsWith('assets:')) {
            displayName = account.substring(7);
        } else if (account.startsWith('liabilities:')) {
            displayName = account.substring(12);
        }
        if (displayName.endsWith(':')) {
            displayName = displayName.substring(0, displayName.length - 1);
        }

        // Update page title and breadcrumb
        document.getElementById('detail-title').textContent = displayName;
        document.getElementById('breadcrumb-type').textContent = 'Account';
        document.getElementById('breadcrumb-name').textContent = displayName;
        document.getElementById('transactions-title').textContent = `${displayName} Transactions`;

        // Store transactions
        detailTransactions = data.transactions || [];
        renderDetailTransactions();

        // Hide breakdown and budget sections for accounts
        document.getElementById('breakdown-section').style.display = 'none';
        document.getElementById('budget-history-section').style.display = 'none';

        // Render balance history
        if (data.balanceHistory && data.balanceHistory.length > 0) {
            renderBalanceHistory(data.balanceHistory);
        }
    } catch (error) {
        console.error('Error loading account detail:', error);
    }
}

// Load income detail
async function loadIncomeDetail(incomeName) {
    try {
        const filterParams = getFilterQueryString();
        const separator = filterParams ? '&' : '';
        const url = `/api/detail/income?income=${encodeURIComponent(incomeName)}${separator}${filterParams}`;
        const response = await fetch(url);
        const data = await response.json();

        // Update page title and breadcrumb
        document.getElementById('detail-title').textContent = incomeName;
        document.getElementById('breadcrumb-type').textContent = 'Income';
        document.getElementById('breadcrumb-name').textContent = incomeName;
        document.getElementById('transactions-title').textContent = `${incomeName} Transactions`;

        // Store transactions
        detailTransactions = data.transactions || [];
        renderDetailTransactions();

        // Render breakdown chart
        if (data.breakdown && data.breakdown.length > 0) {
            // Transform breakdown data to match renderBreakdownChart format
            const transformedBreakdown = data.breakdown.map(item => ({
                name: item.name,
                amount: item.amount
            }));
            renderBreakdownChart(transformedBreakdown, 'Income Breakdown');
        } else {
            document.getElementById('breakdown-section').style.display = 'none';
        }

        // Hide budget history for income (income doesn't have budgets)
        document.getElementById('budget-history-section').style.display = 'none';
    } catch (error) {
        console.error('Error loading income detail:', error);
    }
}

// Load all income detail (aggregate view like tiers)
async function loadAllIncomeDetail() {
    try {
        const filterParams = getFilterQueryString();
        const url = filterParams ? `/api/income-breakdown?${filterParams}` : '/api/income-breakdown';
        const response = await fetch(url);
        const breakdown = await response.json();

        // Update page title and breadcrumb
        document.getElementById('detail-title').textContent = 'All Income';
        document.getElementById('breadcrumb-type').textContent = 'Income';
        document.getElementById('breadcrumb-name').textContent = 'All';
        document.getElementById('transactions-title').textContent = 'All Income Transactions';

        // Fetch all income transactions
        const txFilterParams = getFilterQueryString();
        const txUrl = txFilterParams ? `/api/transactions?${txFilterParams}` : '/api/transactions';
        const txResponse = await fetch(txUrl);
        const allTransactions = await txResponse.json();

        // Filter for income transactions
        const incomeTransactions = allTransactions.filter(tx =>
            tx.tpostings.some(p => p.paccount.startsWith('income:'))
        );

        detailTransactions = incomeTransactions;
        renderDetailTransactions();

        // Render breakdown chart by income category
        if (breakdown && breakdown.length > 0) {
            // Transform breakdown data to match renderBreakdownChart format
            const transformedBreakdown = breakdown.map(item => ({
                name: item.category,
                amount: item.amount
            }));
            renderBreakdownChart(transformedBreakdown, 'Income by Category');
        } else {
            document.getElementById('breakdown-section').style.display = 'none';
        }

        // Hide budget history for income
        document.getElementById('budget-history-section').style.display = 'none';

        // Show income history section
        document.getElementById('income-history-section').style.display = 'block';

        // Load and render income history charts
        loadIncomeHistoryCharts();
    } catch (error) {
        console.error('Error loading all income detail:', error);
    }
}

// Load and render income history charts (similar to budget history)
async function loadIncomeHistoryCharts() {
    try {
        const filterParams = getFilterQueryString();
        const url = filterParams ? `/api/income-history?${filterParams}` : '/api/income-history';
        const response = await fetch(url);
        const items = await response.json();

        const container = document.getElementById('income-history-charts');
        if (!container) return;

        if (!items || items.length === 0) {
            container.classList.remove('budget-charts-grid');
            container.innerHTML = '<p class="budget-empty">No income history available (need at least 2 months of history)</p>';
            return;
        }

        container.classList.add('budget-charts-grid');
        container.innerHTML = '';

        // Render a line chart per income category
        items.forEach((item, idx) => {
            const card = document.createElement('div');
            card.className = 'budget-chart-card';

            const header = document.createElement('div');
            header.className = 'budget-chart-header';
            const avgExcl = item.averageExcludingExtremes || item.average || 0;
            header.innerHTML = `
                <div>
                    <div class="budget-chart-title">${escapeHtml(item.category)}</div>
                    <div class="budget-chart-meta">Avg ${formatCurrency(item.average || 0)} | Excl. extremes ${formatCurrency(avgExcl)}</div>
                </div>
            `;

            const canvas = document.createElement('canvas');
            canvas.className = 'budget-chart-canvas';
            const canvasId = `income-history-chart-${(item.category || 'cat').toLowerCase().replace(/[^a-z0-9]+/g, '-')}-${Math.random().toString(36).slice(2, 6)}`;
            canvas.id = canvasId;
            canvas.height = 200;
            canvas.style.height = '200px';
            canvas.style.width = '100%';

            card.appendChild(header);
            card.appendChild(canvas);
            container.appendChild(card);

            // Group months by year
            const yearGroups = {};
            if (item.months) {
                item.months.forEach(m => {
                    const year = m.year || 'Unknown';
                    if (!yearGroups[year]) {
                        yearGroups[year] = [];
                    }
                    yearGroups[year].push(m);
                });
            }

            // Get all unique month numbers (01-12) for x-axis
            const monthNumbers = new Set();
            if (item.months) {
                item.months.forEach(m => {
                    if (m.month && m.month.length >= 7) {
                        monthNumbers.add(m.month.substring(5, 7));
                    }
                });
            }
            const monthLabels = Array.from(monthNumbers).sort();

            // Create dataset for each year
            const datasets = [];
            const years = Object.keys(yearGroups).sort();

            // Color palette for different years
            const yearColors = [
                '#2ecc71', // Green
                '#27ae60', // Dark green
                '#16a085', // Teal green
                '#1abc9c', // Turquoise
                '#3498db', // Blue
            ];

            years.forEach((year, index) => {
                const yearData = yearGroups[year];
                const monthMap = {};
                yearData.forEach(m => {
                    if (m.month && m.month.length >= 7) {
                        const monthNum = m.month.substring(5, 7);
                        monthMap[monthNum] = m;
                    }
                });

                const dataPoints = monthLabels.map(monthNum => {
                    const entry = monthMap[monthNum];
                    return entry ? entry.amount : null;
                });

                const color = yearColors[index % yearColors.length];
                datasets.push({
                    label: year,
                    data: dataPoints,
                    borderColor: color,
                    backgroundColor: rgbaFromHex(color, 0.1),
                    tension: 0.25,
                    fill: false,
                    pointRadius: 3,
                    pointHoverRadius: 5,
                    spanGaps: true
                });
            });

            // Add average line
            datasets.push({
                label: 'Average',
                data: monthLabels.map(() => item.average || 0),
                borderColor: '#27ae60',
                borderWidth: 2,
                borderDash: [6, 4],
                pointRadius: 0,
                fill: false,
                tension: 0
            });

            // Convert month numbers to readable labels
            const monthNames = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
            const xLabels = monthLabels.map(m => monthNames[parseInt(m) - 1] || m);

            // Set y-axis max to 2x the average
            const yAxisMax = (item.average || 0) * 2;

            const ctx = canvas.getContext('2d');
            const chart = new Chart(ctx, {
                type: 'line',
                data: {
                    labels: xLabels,
                    datasets: datasets
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    scales: {
                        x: {
                            ticks: { autoSkip: true, maxTicksLimit: 12 }
                        },
                        y: {
                            beginAtZero: true,
                            max: yAxisMax > 0 ? yAxisMax : undefined,
                            ticks: {
                                callback: (value) => formatCurrency(value)
                            }
                        }
                    },
                    plugins: {
                        legend: {
                            display: true,
                            position: 'top',
                            labels: {
                                usePointStyle: true,
                                padding: 10,
                                font: { size: 11 }
                            }
                        },
                        tooltip: {
                            callbacks: {
                                label: (ctx) => {
                                    return `${ctx.dataset.label}: ${formatCurrency(ctx.parsed.y || 0)}`;
                                }
                            }
                        }
                    }
                }
            });
        });
    } catch (error) {
        console.error('Error loading income history charts:', error);
        const container = document.getElementById('income-history-charts');
        if (container) {
            container.classList.remove('budget-charts-grid');
            container.innerHTML = '<p class="budget-empty">Error loading income history</p>';
        }
    }
}

// Render breakdown chart (bar chart)
function renderBreakdownChart(breakdown, title) {
    const section = document.getElementById('breakdown-section');
    section.style.display = 'block';

    document.getElementById('breakdown-chart-title').textContent = title;

    const ctx = document.getElementById('breakdownChart');
    if (!ctx) return;

    const labels = breakdown.map(item => item.name);
    const amounts = breakdown.map(item => item.amount);

    // Generate colors
    const colors = labels.map((label, idx) => {
        const hue = (idx * 137.5) % 360; // Golden angle for color distribution
        return `hsl(${hue}, 60%, 55%)`;
    });

    new Chart(ctx, {
        type: 'bar',
        data: {
            labels: labels,
            datasets: [{
                label: 'Amount',
                data: amounts,
                backgroundColor: colors,
                borderColor: colors.map(c => c.replace('55%', '45%')),
                borderWidth: 1
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: true,
            indexAxis: 'y',
            scales: {
                x: {
                    beginAtZero: true,
                    ticks: {
                        callback: function (value) {
                            return '$' + value.toLocaleString();
                        }
                    }
                }
            },
            plugins: {
                legend: {
                    display: false
                },
                tooltip: {
                    callbacks: {
                        label: function (context) {
                            return formatCurrency(context.parsed.x);
                        }
                    }
                }
            }
        }
    });
}

// Render balance history chart
function renderBalanceHistory(balanceHistory) {
    const section = document.getElementById('balance-history-section');
    section.style.display = 'block';

    const ctx = document.getElementById('balanceHistoryChart');
    if (!ctx) return;

    const chartData = balanceHistory.map(point => ({
        x: new Date(point.date),
        y: point.balance
    }));

    new Chart(ctx, {
        type: 'line',
        data: {
            datasets: [{
                label: 'Balance',
                data: chartData,
                borderColor: '#3498db',
                backgroundColor: 'rgba(52, 152, 219, 0.1)',
                borderWidth: 2,
                fill: true,
                tension: 0.1
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: true,
            scales: {
                x: {
                    type: 'time',
                    time: {
                        unit: 'month',
                        displayFormats: {
                            month: 'MMM yyyy'
                        }
                    }
                },
                y: {
                    beginAtZero: false,
                    ticks: {
                        callback: function (value) {
                            return '$' + value.toLocaleString();
                        }
                    }
                }
            },
            plugins: {
                legend: {
                    display: false
                },
                tooltip: {
                    callbacks: {
                        label: function (context) {
                            return formatCurrency(context.parsed.y);
                        }
                    }
                }
            }
        }
    });
}

// Render detail budget history (similar to main dashboard)
function renderDetailBudgetHistory(budgetHistory, isMultiLine = false) {
    const section = document.getElementById('budget-history-section');
    section.style.display = 'block';

    const container = document.getElementById('detail-budget-charts');
    container.innerHTML = '';

    if (!isMultiLine) {
        // Single category - render one chart
        budgetHistory.forEach(item => {
            renderDetailBudgetChart(item);
        });
    } else {
        // Multiple categories (tier) - render multi-line chart
        renderTierBudgetChart(budgetHistory);
    }
}

// Render a single budget chart (for category)
function renderDetailBudgetChart(budgetItem) {
    const container = document.getElementById('detail-budget-charts');

    if (!budgetItem.months || budgetItem.months.length < 2) {
        return;
    }

    const chartCard = document.createElement('div');
    chartCard.className = 'budget-chart-card';

    const header = document.createElement('div');
    header.className = 'budget-chart-header';
    header.innerHTML = `
        <div>
            <div class="budget-chart-title">${budgetItem.category}</div>
            <div class="budget-chart-meta">
                Avg: ${formatCurrency(budgetItem.average)} | 
                Avg (excl. extremes): ${formatCurrency(budgetItem.averageExcludingExtremes)}
            </div>
        </div>
    `;

    const canvas = document.createElement('canvas');
    canvas.className = 'budget-chart-canvas';

    chartCard.appendChild(header);
    chartCard.appendChild(canvas);
    container.appendChild(chartCard);

    // Group by year
    const yearData = {};
    budgetItem.months.forEach(m => {
        if (!yearData[m.year]) {
            yearData[m.year] = [];
        }
        yearData[m.year].push({
            month: m.month,
            amount: m.amount
        });
    });

    const datasets = [];
    const years = Object.keys(yearData).sort();
    const colors = ['#3498db', '#e74c3c', '#2ecc71', '#f39c12', '#9b59b6'];

    years.forEach((year, idx) => {
        const data = yearData[year].map(m => ({
            x: parseMonthToDate(m.month),
            y: m.amount
        }));

        datasets.push({
            label: year,
            data: data,
            borderColor: colors[idx % colors.length],
            backgroundColor: 'transparent',
            borderWidth: 2,
            pointRadius: 3,
            tension: 0.1
        });
    });

    // Add average lines
    const avgLineData = budgetItem.months.map(m => ({
        x: parseMonthToDate(m.month),
        y: budgetItem.average
    }));

    datasets.push({
        label: 'Average',
        data: avgLineData,
        borderColor: '#95a5a6',
        backgroundColor: 'transparent',
        borderWidth: 1,
        borderDash: [5, 5],
        pointRadius: 0,
        tension: 0
    });

    const yMax = Math.max(budgetItem.average * 2, ...budgetItem.months.map(m => m.amount));

    new Chart(canvas, {
        type: 'line',
        data: { datasets },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            scales: {
                x: {
                    type: 'time',
                    time: {
                        unit: 'month',
                        displayFormats: { month: 'MMM yyyy' }
                    }
                },
                y: {
                    beginAtZero: true,
                    max: yMax,
                    ticks: {
                        callback: value => '$' + value.toLocaleString()
                    }
                }
            },
            plugins: {
                legend: {
                    display: true,
                    position: 'top',
                    labels: { boxWidth: 12, font: { size: 10 } }
                },
                tooltip: {
                    callbacks: {
                        label: ctx => `${ctx.dataset.label}: ${formatCurrency(ctx.parsed.y)}`
                    }
                }
            }
        }
    });
}

// Render tier budget chart (multi-category)
function renderTierBudgetChart(budgetHistory) {
    const container = document.getElementById('detail-budget-charts');

    document.getElementById('budget-history-title').textContent = 'Budget History (by Category)';

    // Create one chart with lines for each category plus a total
    const chartCard = document.createElement('div');
    chartCard.className = 'budget-chart-card';
    chartCard.style.height = '400px';

    const canvas = document.createElement('canvas');
    canvas.className = 'budget-chart-canvas';
    canvas.style.height = '380px';
    canvas.style.maxHeight = '380px';

    chartCard.appendChild(canvas);
    container.appendChild(chartCard);

    const datasets = [];
    const colors = ['#3498db', '#e74c3c', '#2ecc71', '#f39c12', '#9b59b6', '#1abc9c', '#e67e22'];

    // Add a dataset for each category
    budgetHistory.forEach((item, idx) => {
        const data = item.months.map(m => ({
            x: parseMonthToDate(m.month),
            y: m.amount
        }));

        datasets.push({
            label: item.category,
            data: data,
            borderColor: colors[idx % colors.length],
            backgroundColor: 'transparent',
            borderWidth: 2,
            pointRadius: 2,
            tension: 0.1
        });
    });

    // Calculate total line
    const allMonths = {};
    budgetHistory.forEach(item => {
        item.months.forEach(m => {
            if (!allMonths[m.month]) {
                allMonths[m.month] = 0;
            }
            allMonths[m.month] += m.amount;
        });
    });

    const totalData = Object.keys(allMonths).sort().map(month => ({
        x: parseMonthToDate(month),
        y: allMonths[month]
    }));

    datasets.push({
        label: 'Total',
        data: totalData,
        borderColor: '#2c3e50',
        backgroundColor: 'transparent',
        borderWidth: 3,
        pointRadius: 3,
        tension: 0.1
    });

    new Chart(canvas, {
        type: 'line',
        data: { datasets },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            scales: {
                x: {
                    type: 'time',
                    time: {
                        unit: 'month',
                        displayFormats: { month: 'MMM yyyy' }
                    }
                },
                y: {
                    beginAtZero: true,
                    ticks: {
                        callback: value => '$' + value.toLocaleString()
                    }
                }
            },
            plugins: {
                legend: {
                    display: true,
                    position: 'top',
                    labels: { boxWidth: 12, font: { size: 11 } }
                },
                tooltip: {
                    callbacks: {
                        label: ctx => `${ctx.dataset.label}: ${formatCurrency(ctx.parsed.y)}`
                    }
                }
            }
        }
    });
}
