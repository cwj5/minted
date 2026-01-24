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
                <div class="account-name">${escapeHtml(account.name)}</div>
                <div class="account-balance">${formatCurrency(account.balance || 0)}</div>
            </div>
        `).join('');
    } catch (error) {
        console.error('Error loading accounts:', error);
        document.getElementById('accounts').innerHTML = '<p>Error loading accounts</p>';
    }
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
        
        container.innerHTML = transactions.map(tx => `
            <div class="transaction-item">
                <div class="transaction-left">
                    <div class="transaction-date">${formatDate(tx.date)}</div>
                    <div class="transaction-description">${escapeHtml(tx.description)}</div>
                    <div class="transaction-account">
                        ${tx.postings.map(p => escapeHtml(p.account)).join(', ')}
                    </div>
                </div>
                <div class="transaction-amount">
                    ${formatCurrency(tx.postings[0]?.amount || 0)}
                </div>
            </div>
        `).join('');
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
    loadAccounts();
    loadTransactions();
});
