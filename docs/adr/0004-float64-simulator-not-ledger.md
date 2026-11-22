# float64 for prices and money: Kanso is a simulator, not a ledger

Prices, indicators, cost and slippage percentages, cash and equity curves are all `float64`. Share and lot quantities are `int64`. Split-adjusted prices aren't exact paise anyway, so decimal types would add allocations and complexity without making anything exact, and they would break parity with the vectorbt oracle, which also uses float64. Tests compare against the oracle with a 1e-9 relative tolerance, and the UI rounds ₹ to 2 decimal places for display.

## Considered Options

- **Integer paise for cash**: correct for a ledger, but it needs conversions at every fill and causes small mismatches with the oracle.
- **shopspring/decimal everywhere**: slow over millions of bars, and still doesn't make adjusted prices exact.
