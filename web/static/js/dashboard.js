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
                    p.paccount.startsWith('Assets:') || p.paccount.startsWith('Liabilities:')
                );
                if (assetLiability) {
                    account = getAccount(assetLiability.paccount);
                }

                // Find first Expenses or Income posting
                const expenseIncome = tx.tpostings.find(p => 
                    p.paccount.startsWith('Expenses:') || p.paccount.startsWith('Income:')
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

// Initialize dashboard
document.addEventListener('DOMContentLoaded', () => {
    loadSummary();
    loadBudgetData();
    loadAccounts();
    loadTransactions();
});
