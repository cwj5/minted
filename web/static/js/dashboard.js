// Format currency
function formatCurrency(amount) {
    return new Intl.NumberFormat('en-US', {
        style: 'currency',
        currency: 'USD'
    }).format(amount);
}

// Store settings globally for color mapping
let appSettings = null;
let budgetCharts = [];

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
            loadAccounts(),
            loadTransactions(),
            loadIncomeExpensesChart(),
            loadCategorySpendingChart(),
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
        const response = await fetch('/api/summary');
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
        const response = await fetch('/api/accounts');
        const accounts = await response.json();

        const container = document.getElementById('accounts');
        if (!accounts || accounts.length === 0) {
            container.innerHTML = '<p>No accounts found</p>';
            return;
        }

        container.innerHTML = accounts.map(account => `
            <div class="account-item">
                <div class="account-name">${escapeHtml(account.aname)}</div>
                <div class="account-balance">${formatCurrency(account.aebalance || 0)}</div>
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
        const response = await fetch('/api/budget/history');
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
        const response = await fetch('/api/transactions');
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

// Load and render income vs expenses chart
async function loadIncomeExpensesChart() {
    try {
        const response = await fetch('/api/monthly-metrics');
        const metrics = await response.json();

        if (!metrics || metrics.length === 0) {
            return;
        }

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
        const response = await fetch('/api/category-spending');
        const spending = await response.json();

        if (!spending || spending.length === 0) {
            return;
        }

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

    loadSettings().then(() => {
        loadSummary();
        loadBudgetHistory();
        loadAccounts();
        loadTransactions();
        loadIncomeExpensesChart();
        loadCategorySpendingChart();
        loadNetWorthChart();
        loadCategoryTrendsChart();
        loadYearOverYearChart();
    });
});

// Convert YYYY-MM to Date for chart display
function parseMonthToDate(monthStr) {
    const [year, month] = monthStr.split('-');
    return new Date(year, parseInt(month) - 1, 1);
}

// Load and render net worth over time chart
async function loadNetWorthChart() {
    try {
        const response = await fetch('/api/net-worth-over-time');
        const data = await response.json();

        if (!data || data.length === 0) {
            return;
        }

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
        const response = await fetch('/api/category-trends');
        const data = await response.json();

        if (!data || data.length === 0) {
            return;
        }

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
        const response = await fetch('/api/year-over-year-comparison');
        const data = await response.json();

        if (!data || data.length === 0) {
            return;
        }

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

