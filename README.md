# Minted - hledger Dashboard

A modern web dashboard for visualizing your financial data from [hledger](https://hledger.org/), a plaintext accounting tool.

## Features

- 📊 **Account Overview** - View all Assets and Liabilities accounts with current balances
- 💳 **Credit Card Tracking** - Monitor credit card balances
- 📈 **Transaction History** - Browse all transactions with dates, descriptions, and amounts
- 🔗 **Live Data** - All data is pulled directly from your hledger journal file in real-time

## Prerequisites

- [Go 1.16+](https://golang.org/doc/install)
- [hledger](https://hledger.org/install) installed and in your PATH
- A hledger journal file (default: `~/test.journal`)

## Quick Start

1. **Clone the repository**
   ```bash
   cd /Users/cwj5/software/minted
   ```

2. **Create a test journal file** (if needed)
   ```bash
   cp ~/test.journal ~/test.journal.backup  # Backup if exists
   ```
   The repo includes sample transactions in `~/test.journal`

3. **Build and run**
   ```bash
   make run
   ```
   The dashboard will start on `http://localhost:9999`

4. **View the dashboard**
   Open your browser to `http://localhost:9999`

## Project Structure

```
minted/
├── cmd/minted/              # Application entry point
│   └── main.go              # Server setup and configuration
├── internal/
│   ├── dashboard/           # API handlers and service logic
│   │   └── service.go       # Dashboard business logic
│   └── hledger/             # hledger integration
│       └── parser.go        # JSON parsing for hledger output
├── web/
│   ├── static/              # Frontend assets
│   │   ├── css/style.css    # Dashboard styling
│   │   └── js/dashboard.js  # Frontend logic
│   └── templates/           # HTML templates
│       └── dashboard.html   # Main dashboard page
├── .env                     # Environment configuration
├── go.mod / go.sum          # Go dependencies
└── Makefile                 # Build and run commands
```

## Configuration

Edit `.env` to customize:
```
PORT=9999                     # Server port
HLEDGER_FILE=~/test.journal  # Path to your hledger journal
```

## Available Commands

```bash
make help       # Show all available commands
make deps       # Install dependencies
make build      # Build the application
make run        # Build and run
make test       # Run tests
make clean      # Clean build artifacts
make kill       # Kill any running processes
```

## API Endpoints

- `GET /` - Dashboard page
- `GET /api/accounts` - List accounts (Assets & Liabilities only)
- `GET /api/transactions` - List all transactions
- `GET /api/summary` - Financial summary (net worth, totals)

## Hledger Integration

The dashboard uses hledger's JSON output to fetch financial data:

- **Accounts**: `hledger balance --empty -O json`
  - Filters to Assets and Liabilities accounts only
  - Includes zero-balance accounts (useful for credit cards)
  
- **Transactions**: `hledger print -O json`
  - Retrieves all transactions with full details
  - Parsed to extract dates, descriptions, and amounts

## Development Notes

### Current Implementation Status

✅ **Completed:**
- Basic dashboard layout with accounts and transactions
- Assets and Liabilities account filtering
- Transaction display with parsing
- Credit card support (zero-balance accounts)
- Environment variable configuration
- Makefile with common tasks

### TODO / Future Enhancements

1. **Budget Tracking Section**
   - Display Expenses and Income categories separately
   - Show monthly spending vs budget
   - Add budget comparison charts
   
2. **Monthly Summary**
   - Calculate monthly income/expenses
   - Show trends over time
   - Category breakdowns

3. **Enhanced Dashboard**
   - Add date range filters
   - Search/filter transactions
   - Account detail pages
   - Charts and graphs (income vs expenses)

4. **Account Reconciliation**
   - Mark transactions as reconciled
   - Show unreconciled items
   - Reconciliation status for each account

5. **Improvements**
   - Error handling and UI feedback
   - Settings page for configuration
   - Multiple journal file support
   - Import/export functionality
   - Mobile responsive design improvements

6. **Testing**
   - Unit tests for parser
   - Integration tests for API
   - Frontend test coverage

### Implementation Tips for Future Sessions

**To add a new expense category section:**
1. Add new method `GetExpenses()` in `internal/hledger/parser.go`
   - Filter accounts with `strings.HasPrefix(name, "Expenses:")`
   - Use similar JSON parsing as `GetAccounts()`
   
2. Add new endpoint in `internal/dashboard/service.go`
   ```go
   router.GET("/api/expenses", dashService.HandleExpenses)
   ```

3. Add frontend section in `web/templates/dashboard.html`

4. Add fetch logic in `web/static/js/dashboard.js`

**To add charts/graphs:**
- Consider Chart.js or similar library
- Calculate monthly aggregates from transactions
- Store temporary data in frontend state

**Testing hledger output:**
```bash
# View all accounts
hledger -f ~/test.journal accounts

# Check specific account balance
hledger -f ~/test.journal balance Assets:Checking

# Validate journal
hledger -f ~/test.journal check

# Export as JSON
hledger -f ~/test.journal print -O json
```

**Common Issues:**

1. **Liabilities account not showing**: Use `--empty` flag in balance command to include zero-balance accounts
2. **Unbalanced transactions**: Ensure each transaction has postings that sum to zero
3. **Port already in use**: Run `make kill` to stop any running processes
4. **File path issues**: Use `~/` for home directory paths in `.env`, code handles expansion

## Sample Journal Structure

The test journal includes:
- Opening balance
- Direct bank transactions (checking, savings)
- Credit card purchases and payments
- Various expense categories (Food, Transport, Utilities, Shopping, Entertainment)
- Income transactions

See `~/test.journal` for the complete sample.

## License

MIT

## Related Resources

- [hledger Documentation](https://hledger.org/)
- [hledger Web API](https://hledger.org/hledger-web.html#json-api)
- [Go Web Development](https://golang.org/doc/effective_go)
- [Gin Web Framework](https://gin-gonic.com/)