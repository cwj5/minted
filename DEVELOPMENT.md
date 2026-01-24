# Development Guide - Minted Dashboard

Complete technical reference for continuing development on the Minted project.

## Architecture Overview

### Data Flow

```
hledger journal file (~/test.journal)
           ↓
    hledger CLI commands
      (balance, print)
           ↓
    os/exec in Go
    (parser.go)
           ↓
    JSON unmarshaling
    (Account, Transaction structs)
           ↓
    REST API Endpoints
    (/api/accounts, /api/transactions)
           ↓
    Fetch API in JavaScript
    (dashboard.js)
           ↓
    DOM updates in HTML
    (dashboard.html)
```

### Component Responsibilities

**cmd/minted/main.go**
- Initialize Gin web server
- Load and parse .env configuration file
- Expand tilde (`~`) in file paths to user home directory
- Register HTTP routes for dashboard and API
- Start listening on configured port

**internal/hledger/parser.go**
- Execute hledger CLI commands as subprocesses
- Parse JSON output from hledger
- Transform hledger data structures into API models
- Apply business logic filters (Assets/Liabilities only)
- Handle errors and edge cases

**internal/dashboard/service.go**
- Define HTTP handlers for each API endpoint
- Call parser methods to fetch data
- Return properly formatted JSON responses
- Handle request parameters (e.g., limit on transactions)

**web/** 
- HTML templates rendered by Gin
- CSS styling with Bootstrap framework
- JavaScript for client-side fetching and rendering
- Build API request URLs and handle responses

## Key Technical Details

### hledger JSON Format

hledger outputs complex nested JSON structures. Understanding these is critical:

**Balance Command Output** (`hledger balance --empty -O json`)
```json
[
  [
    {"aname": "Assets:Checking", "aebalance": {...}, ...},
    {"aname": "Assets:Savings", "aebalance": {...}, ...},
    ...
  ],
  [
    {"aname": "Total", "aebalance": {...}, ...}
  ]
]
```

The structure is `[[account_entries], [totals]]` - two separate arrays.

**Amount Structure** (in aebalance and pamount)
```json
{
  "acommodity": "$",
  "aquantity": {
    "decimalMantissa": 500000,
    "decimalExponent": -2
  }
}
```

To convert: `quantity = decimalMantissa * 10^decimalExponent` = `500000 * 10^-2` = `5000.00`

**Print Command Output** (`hledger print -O json`)
```json
[
  {
    "tdate": "2024-01-15",
    "tdescription": "Grocery shopping",
    "tpostings": [
      {
        "paccount": "Expenses:Food",
        "pamount": {
          "acommodity": "$",
          "aquantity": {...}
        }
      },
      {
        "paccount": "Assets:Checking",
        "pamount": {...}
      }
    ]
  },
  ...
]
```

### Important Code Sections

**File Path Expansion** (cmd/minted/main.go)
```go
if strings.HasPrefix(path, "~") {
    homeDir, _ := user.Current().HomeDir
    path = strings.Replace(path, "~", homeDir, 1)
}
```
This allows using `~/test.journal` in .env files.

**Account Filtering** (internal/hledger/parser.go)
```go
if strings.HasPrefix(account.Name, "Assets:") || 
   strings.HasPrefix(account.Name, "Liabilities:") {
    // Include account
}
```
Only shows bank accounts and credit cards, hides expense categories.

**Decimal Conversion** (internal/hledger/parser.go)
```go
mantissa := int64(q.DecimalMantissa)
exponent := q.DecimalExponent
balance := float64(mantissa) * math.Pow(10, float64(exponent))
```
Converts hledger's fixed-point representation to floating-point.

**Zero-Balance Accounts** (internal/hledger/parser.go)
```bash
hledger balance --empty -O json
```
The `--empty` flag is crucial for showing credit cards with zero balance after payment.

## Testing & Debugging

### Verify hledger Installation
```bash
hledger --version
hledger -f ~/test.journal balance
```

### Test API Endpoints Directly
```bash
# Start server
make run

# In another terminal
curl http://localhost:9999/api/accounts | jq
curl http://localhost:9999/api/transactions | jq
```

### Verify Journal Integrity
```bash
hledger -f ~/test.journal check
```

### Debug JavaScript Fetch Errors
1. Open browser DevTools (F12)
2. Go to Network tab
3. Check API endpoint responses
4. Verify JSON structure matches expected format

### Common Issues & Solutions

**Issue**: API returns `[]` or `null` for accounts
- **Cause**: hledger file path not found or invalid
- **Solution**: Check `.env` path, verify file exists: `ls ~/test.journal`

**Issue**: "error loading the transactions" in dashboard
- **Cause**: JSON parsing error in JavaScript
- **Solution**: Check browser console, verify `tdate`, `tdescription`, `tpostings` fields exist

**Issue**: Credit card account doesn't appear
- **Cause**: hledger doesn't show zero-balance accounts by default
- **Solution**: Ensure `--empty` flag in GetAccounts() command

**Issue**: Amounts showing as extremely large numbers
- **Cause**: Not converting decimalMantissa/decimalExponent
- **Solution**: Apply `mantissa * 10^exponent` calculation in JavaScript

**Issue**: Port already in use
- **Solution**: `make kill` to stop all running `minted` processes

## Future Development Workflow

### Adding a New Endpoint

1. **Define data structure** in `internal/hledger/parser.go`
   ```go
   type MyData struct {
       Field1 string `json:"field1"`
       Field2 float64 `json:"field2"`
   }
   ```

2. **Add parser method** in `internal/hledger/parser.go`
   ```go
   func (h *HLedger) GetMyData() ([]MyData, error) {
       // Execute hledger command, parse JSON, return data
   }
   ```

3. **Add HTTP handler** in `internal/dashboard/service.go`
   ```go
   func (ds *DashboardService) HandleMyData(c *gin.Context) {
       data, err := ds.hledger.GetMyData()
       c.JSON(http.StatusOK, data)
   }
   ```

4. **Register route** in `cmd/minted/main.go`
   ```go
   api.GET("/mydata", dashService.HandleMyData)
   ```

5. **Update frontend** in `web/static/js/dashboard.js`
   ```javascript
   fetch('/api/mydata')
       .then(r => r.json())
       .then(data => { /* render to DOM */ })
   ```

### Adding hledger Filters

hledger commands support powerful filtering:
```bash
hledger -f ~/test.journal balance -p "this month"
hledger -f ~/test.journal print "tag:invoice"
hledger -f ~/test.journal accounts --tree
```

To add date range filtering:
1. Accept query parameters in Go handlers
2. Build hledger command with `-p` flag for periods
3. Re-execute parser with date filters

### Performance Considerations

- **Caching**: Currently all data is fetched on each request. For large journals, consider caching parsed data.
- **Limits**: Transactions endpoint supports `limit` parameter to avoid returning thousands of items.
- **Optimization**: For monthly views, pre-compute aggregates rather than summing in JavaScript.

## Project Dependencies

### Go Packages
- `github.com/gin-gonic/gin` - Web framework
- `github.com/joho/godotenv` - .env file parsing
- Standard library: `encoding/json`, `os/exec`, `strings`, `os/user`, `math`

### Frontend
- Bootstrap (CSS framework via CDN)
- Vanilla JavaScript (no additional frameworks)

### System Requirements
- hledger v1.50+ (with JSON output support)

## Deployment Notes

### Building for Production
```bash
make build
./bin/minted
```

### Environment Configuration
Create `.env` with production paths:
```
PORT=9999
HLEDGER_FILE=/path/to/production/journal.journal
```

### Running as a Service
On macOS with launchd or systemd on Linux to keep the service running.

## Known Limitations

1. **Summary endpoint** (`/api/summary`) currently returns hardcoded values
   - Should calculate from GetAccounts() balances
   - Need to sum Assets and Liabilities separately

2. **No expense/income tracking** in main dashboard
   - Currently only shows Assets and Liabilities
   - Expense categories hidden for simplicity

3. **No date filtering** on transactions
   - All transactions always shown (with limit)
   - Could add `?start=YYYY-MM-DD&end=YYYY-MM-DD` parameters

4. **No transaction reconciliation**
   - Can't mark transactions as cleared in UI
   - Would require separate reconciliation file or hledger integration

5. **Limited error handling**
   - No validation of hledger command failures
   - Frontend doesn't retry on network errors

## Next Priority Features

### High Priority
1. **Implement HandleSummary endpoint**
   - Calculate totals from account balances
   - Location: `internal/dashboard/service.go`
   - Logic: sum Assets, sum Liabilities, calculate Net Worth

2. **Add more account filtering options**
   - Show Expenses/Income if needed
   - Add checkboxes or filters on dashboard

### Medium Priority
1. **Add date range filtering to transactions**
   - Parameter parsing and hledger `-p` flag usage

2. **Add monthly summary/trends**
   - Aggregate transactions by month
   - Show spending patterns

3. **Responsive design improvements**
   - Mobile-friendly layouts
   - Touch-friendly interactions

### Lower Priority
1. Charts and visualizations (Chart.js or D3.js)
2. Budget comparison features
3. Multiple journal support
4. User authentication
5. Export to CSV/PDF

## Debugging Checklist

Before assuming a bug:
- [ ] Verify hledger is installed: `hledger --version`
- [ ] Check journal is valid: `hledger -f ~/test.journal check`
- [ ] Verify file path in `.env`: `cat .env`
- [ ] Check API response: `curl -s http://localhost:9999/api/accounts | jq`
- [ ] Check browser console for JavaScript errors (F12)
- [ ] Verify server is running: `ps aux | grep minted`
- [ ] Try killing and restarting: `make kill && make run`
- [ ] Check if port is already in use: `lsof -i :9999`

## Code Style & Conventions

- **Go**: Follow standard Go conventions, use `gofmt`
- **JavaScript**: Use arrow functions, const/let over var
- **HTML/CSS**: Use semantic HTML, Bootstrap utility classes
- **Git commits**: Descriptive messages, one logical change per commit

## Repository State

**Last Known Working State**:
- All accounts and transactions loading correctly
- Assets/Liabilities filtering working
- Credit card balances showing (with --empty flag)
- Test journal with realistic transactions
- Dashboard displaying data without errors

**To resume work**:
1. Pull latest changes
2. Run `make run` to start server
3. Navigate to `http://localhost:9999`
4. Check browser console for any errors
5. Review changes since last session with `git log`

## Questions for Next Session

When continuing this project:
1. What feature is the priority? (Summary, Expenses, Charts, etc.)
2. Is the hledger file path still `~/test.journal` or different?
3. Are there new accounts or transactions to add?
4. Any specific UI improvements needed?
5. Performance issues with large journals?
