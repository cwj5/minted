// Format currency
function formatCurrency(amount) {
    return new Intl.NumberFormat('en-US', {
        style: 'currency',
        currency: 'USD'
    }).format(amount);
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

// Get day of month percentage (how much of the month is complete)
function getDayOfMonthPercent() {
    const now = new Date();
    const lastDay = new Date(now.getFullYear(), now.getMonth() + 1, 0).getDate();
    return (now.getDate() / lastDay) * 100;
}

// Load budget data
async function loadBudgetData() {
    try {
        const response = await fetch('/api/budget');
        const budgetItems = await response.json();

        const container = document.getElementById('budget');
        if (!budgetItems || budgetItems.length === 0) {
            container.innerHTML = '<p>No budget data available (need at least 2 months of history)</p>';
            return;
        }

        const dayPercent = getDayOfMonthPercent();

        container.innerHTML = budgetItems.map(item => {
            // Determine color class based on percent budget
            let statusClass = 'on-budget';
            if (item.percentBudget > 100) {
                statusClass = 'over-budget';
            } else if (item.percentBudget > 80) {
                statusClass = 'near-budget';
            }

            // Determine status text
            let statusText = 'On track';
            if (item.percentBudget > 100) {
                statusText = `Over by ${formatCurrency(item.variance)}`;
            } else if (item.variance > 0) {
                statusText = `Over by ${formatCurrency(item.variance)}`;
            } else {
                statusText = `Under by ${formatCurrency(-item.variance)}`;
            }

            return `
            <div class="budget-item ${statusClass}">
                <div class="budget-header">
                    <div class="budget-category">${escapeHtml(item.category)}</div>
                    <div class="budget-status">${statusText}</div>
                </div>
                <div class="budget-amounts">
                    <div class="budget-row">
                        <span class="label">Average:</span>
                        <span class="amount">${formatCurrency(item.average)}</span>
                    </div>
                    <div class="budget-row">
                        <span class="label">This Month:</span>
                        <span class="amount">${formatCurrency(item.currentMonth)}</span>
                    </div>
                </div>
                <div class="budget-progress-container">
                    <div class="budget-progress-bar">
                        <div class="budget-pace-line" style="left: ${dayPercent}%"></div>
                        <div class="budget-progress-fill" style="width: ${Math.min(item.percentBudget, 100)}%"></div>
                    </div>
                    <div class="budget-percent">${item.percentBudget.toFixed(0)}%</div>
                </div>
            </div>
        `}).join('');
    } catch (error) {
        console.error('Error loading budget data:', error);
        document.getElementById('budget').innerHTML = '<p>Error loading budget data</p>';
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
async function loadTransactions() {
    try {
        const response = await fetch('/api/transactions');
        const transactions = await response.json();

        const container = document.getElementById('transactions');
        if (!transactions || transactions.length === 0) {
            container.innerHTML = '<p>No transactions found</p>';
            return;
        }

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

        html += transactions.map(tx => {
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
    } catch (error) {
        console.error('Error loading transactions:', error);
        document.getElementById('transactions').innerHTML = '<p>Error loading transactions</p>';
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

        const labels = Object.keys(categoryTotals).sort();
        const data = labels.map(cat => categoryTotals[cat]);

        // Color palette
        const colors = [
            '#3498db', '#e74c3c', '#2ecc71', '#f39c12', '#9b59b6',
            '#1abc9c', '#34495e', '#e67e22', '#95a5a6', '#16a085'
        ];

        new Chart(ctx, {
            type: 'doughnut',
            data: {
                labels: labels,
                datasets: [{
                    data: data,
                    backgroundColor: colors.slice(0, labels.length),
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
    loadSummary();
    loadBudgetData();
    loadAccounts();
    loadTransactions();
    loadIncomeExpensesChart();
    loadCategorySpendingChart();
    loadNetWorthChart();
    loadCategoryTrendsChart();
    loadYearOverYearChart();
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

        // Color palette
        const colors = [
            '#3498db', '#e74c3c', '#2ecc71', '#f39c12', '#9b59b6',
            '#1abc9c', '#34495e', '#e67e22', '#95a5a6', '#16a085'
        ];

        const datasets = data.map((cat, idx) => {
            const chartData = cat.data.map(ma => ({
                x: parseMonthToDate(ma.month),
                y: ma.amount
            }));

            return {
                label: cat.category,
                data: chartData,
                borderColor: colors[idx % colors.length],
                backgroundColor: colors[idx % colors.length],
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

