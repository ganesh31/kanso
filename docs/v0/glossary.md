# Trading jargon cheat sheet

Plain-English definitions for every term that comes up in Kanso. Sections run roughly from basics to advanced. Examples use real NSE stocks from V0 where possible.

**🔷 = matters directly for Kanso V1**

---

## 1. Market basics

| Term | What it means |
|---|---|
| **NSE / BSE** | India's two stock exchanges: the National Stock Exchange and the Bombay Stock Exchange. Most trading happens on NSE. 🔷 |
| **Stock / share / equity** | A small piece of ownership in a company. "Equity" also means stocks as a category. |
| **Scrip** | Indian broker jargon for a single stock. "Scrip report" = a report on one stock. |
| **Symbol / ticker** | A stock's short code, e.g. `RELIANCE`. Yahoo adds `.NS` for NSE (`RELIANCE.NS`) and TradingView adds a prefix (`NSE:RELIANCE`). 🔷 |
| **Index** | A number tracking a basket of stocks, like a "market average". It isn't a stock you can buy directly. 🔷 |
| **Nifty 50** | Index of the 50 largest NSE companies, the standard measure of how "the market" did. Yahoo ticker `^NSEI`. Kanso compares against it (the benchmark). 🔷 |
| **Nifty 500** | Index of the 500 largest NSE companies. Kanso's default stock list. 🔷 |
| **Constituents** | The stocks currently inside an index. They change over time as companies are added and removed. 🔷 |
| **Universe / stock list** | The set of stocks a scan or backtest runs over, e.g. "Nifty 500". 🔷 |
| **Large / mid / small cap** | Company size by market value ("market capitalisation"). Small caps move more and are riskier. |
| **Sector** | Industry group, e.g. banking, IT, pharma. |
| **Demat account** | The account that holds your shares electronically. You need one to buy stocks in India. |
| **Broker** | The company that places your orders on the exchange (Zerodha, Definedge, etc.). |
| **Market hours** | NSE trades 9:15 am – 3:30 pm IST, Monday to Friday, except holidays. |
| **EOD (end of day)** | Data published after the market closes: one bar per stock per day. Kanso V1 uses EOD only. 🔷 |
| **Bhavcopy** | NSE's official end-of-day price file for all stocks. eod2 builds its data from it. |
| **Trading day** | A day the exchange is open. A year has about **252** of them, which is where Kanso's "3 years = 756 bars" comes from. 🔷 |

## 2. Price data and charts

| Term | What it means |
|---|---|
| **OHLC / OHLCV** | Open, High, Low, Close (+ Volume) for a period. This is the raw data Kanso stores. 🔷 |
| **Open** | First traded price of the period. Kanso buys at the **next day's open** after a signal. 🔷 |
| **High / Low** | Highest and lowest price traded during the period. |
| **Close** | Last price of the period. Most signals use the close, because it's the "settled" price for the day. 🔷 |
| **Volume** | How many shares changed hands during the period. 🔷 |
| **Bar** | One period's OHLCV, whether a day, a week or a month. "756 bars" = 756 trading days on a daily chart. 🔷 |
| **Candle / candlestick** | The usual way to draw a bar. The thick **body** spans open to close (green if close > open, red if lower), and thin **wicks** reach to the high and low. |
| **Line chart** | Just the closes joined by a line. |
| **Timeframe** | How long each bar covers: daily, weekly, monthly. A **weekly** bar = Monday's open, the week's high and low, Friday's close, and the total volume. 🔷 |
| **Gap up / gap down** | Today opens well above (or below) yesterday's close, with no trading in between. It matters for stop-losses (see §6). 🔷 |
| **Intraday** | Within a single trading day, e.g. 5-minute bars. Not used in Kanso V1. |

## 3. Adjustments (why old prices change)

| Term | What it means |
|---|---|
| **Corporate action** | Something a company does that changes its share count or price: a split, a bonus or a dividend. |
| **Stock split** | One share becomes several, e.g. 1 share of ₹1,000 → 2 shares of ₹500. The company's value doesn't change, only the price per share. 🔷 |
| **Bonus issue** | Free extra shares to existing holders, e.g. 1:1. The effect on the price is like a split. 🔷 |
| **Dividend** | Cash the company pays shareholders. The price drops by about that amount on the "ex-date". |
| **Adjusted prices** | Historical prices rescaled so splits and bonuses don't show up as fake crashes. Without this, a 1:2 split looks like a 50% fall, and old highs look twice as high as they really were. Kanso uses **split-adjusted** prices. 🔷 |

## 4. Levels, patterns and breakouts

| Term | What it means |
|---|---|
| **Resistance / ceiling** | A price level where selling has repeatedly stopped the stock from going higher. 🔷 |
| **Support / floor** | A price level where buying has repeatedly stopped the stock from falling. |
| **All-time high (ATH)** | The highest price ever. Kanso's "all-time" setting needs at least 2 years of history. 🔷 |
| **52-week high** | Highest price in the last year. The most common version of "new high". |
| **N-year high** | Highest *high* in the last N years. Kanso's core pattern: today's close is above the highest high of the prior N years. 🔷 |
| **Breakout** | The price closes above a resistance level. A **multi-year breakout** clears a ceiling that has held for years. Example: DIVISLAB, Sept 2024, cleared its Oct 2021 high of ₹5,425. 🔷 |
| **Breakout level** | The exact price being broken, e.g. ₹5,425. Kanso draws it as a line on the chart. 🔷 |
| **Breakdown** | The mirror image of a breakout: a close below support. |
| **Base / consolidation** | A long sideways stretch under a ceiling. Longer bases often lead to stronger breakouts. DIVISLAB's base ran 2022–2024. 🔷 |
| **Overhead supply** | People who bought near the old high and are waiting to sell once they're back to even. This is why ceilings hold, and why clearing one matters. |
| **False breakout / fakeout** | The stock pokes above the level and then falls back below it. This is what filters and stop-losses guard against. 🔷 |
| **Close confirmation** | Waiting for a close above the level, daily or **weekly**, rather than just a brief poke above it during the day. 🔷 |
| **Extended** | The price has already run far above the breakout level, so buying now is late and the stop is far away. Kanso's "not extended" filter: no more than 8% above the level. 🔷 |
| **Continuation high** | A new high in a stock that's already rising, where the "old high" is only days old. It isn't a base breakout, and V0 found far more of these than real breakouts. 🔷 |
| **Retest** | After a breakout, the price dips back to the old ceiling, which now acts as support, before moving up again. |
| **Swing high / swing low (pivot)** | A local peak or trough on the chart. You need these to detect patterns like double tops. (V2) |
| **Tested ceiling** | A resistance level the price has touched several times. More touches mean more meaningful resistance. (V2 pattern) |
| **Double top / triple top** | Two or three peaks at about the same level. Breaking above them is a "double top buy" in Point & Figure terms. |
| **Cup and handle** | A rounded "U" base followed by a small dip, then a breakout. |
| **Triangle** | Highs falling and/or lows rising, so the range narrows toward a breakout. |
| **VCP (volatility contraction pattern)** | Swings that get smaller and tighter before a breakout (from Mark Minervini). Skipped in V1 because it's hard to define objectively. |
| **NR7** | "Narrowest range of the last 7 days", a sign of calm before a move. |

## 5. Volume, strength and indicators

| Term | What it means |
|---|---|
| **Average volume** | Typical daily volume, e.g. the 50-day average. |
| **Volume surge** | Volume well above average, e.g. **2×** the 50-day average. A breakout on a surge suggests big buyers are involved. INFY's failed 2024 breakout came on 0.73×, below average. 🔷 |
| **Relative strength (RS)** | How a stock did *compared with* the market. Kanso's version: the stock's 6-month return minus the Nifty 50's. Positive = it's beating the market. 🔷 |
| **RSI** | *Different from RS!* The "Relative Strength Index", a 0–100 momentum gauge of recent ups vs downs. Above 70 = "overbought", below 30 = "oversold". Not used in V1. |
| **Momentum** | The tendency of stocks that have been rising to keep rising for a while. Breakout trading is a form of momentum trading. |
| **Leader / laggard** | Stocks beating the market vs lagging it. |
| **Market breadth** | How many stocks are rising vs falling overall: a health check for the whole market. |
| **Moving average (MA / SMA)** | The average close over the last N bars, drawn as a smooth line. **SMA** = simple moving average. Kanso's trend exit: the weekly close falls below the **30-week SMA**. 🔷 |
| **EMA** | Exponential moving average: like an SMA, but recent prices count more. |
| **ATR** | Average True Range: how much a stock typically moves per bar. Used for volatility-based stops and Renko brick sizes. |
| **Volatility** | How much a price swings. Higher volatility means bigger moves both ways. |

## 6. Trading and execution

| Term | What it means |
|---|---|
| **Long / short** | Long = you buy, hoping the price rises. Short = you sell first, hoping it falls. Kanso V1 is long-only. |
| **Position** | A holding you currently have, e.g. "100 shares of DIVISLAB". 🔷 |
| **Entry / exit** | Buying into and selling out of a position. 🔷 |
| **Signal** | The moment a rule says "buy" or "sell". In Kanso, a signal fires on the **first** day the rule becomes true ("edge-triggered"), not every day it stays true. 🔷 |
| **Fill / fill price** | The price you actually got. Kanso assumes the **next day's open**, because you can't trade at the close you only just saw. 🔷 |
| **Market order** | "Buy now at whatever the price is". The fill can be worse than the price on screen. |
| **Slippage** | The gap between the price you expected and the price you got. Kanso assumes 0.10% per side. 🔷 |
| **Brokerage and charges** | Broker fees plus government charges (STT, stamp duty, exchange fees). Kanso assumes 0.20% per side in total. 🔷 |
| **STT** | Securities Transaction Tax, an Indian tax on each trade. |
| **Per side / round trip** | "Per side" = buying OR selling. A round trip = buy + sell, so it costs twice the per-side cost. 🔷 |
| **Stop-loss** | A pre-set exit if the price falls to a certain level, which limits the loss. Kanso default: 8% below your buy price. 🔷 |
| **Gap through the stop** | The stock opens *below* your stop, so you get out at the (worse) open price, not at your stop price. Kanso models this honestly. 🔷 |
| **Trailing stop / trend exit** | An exit that rises as the price rises, so winners keep running and you sell only when the trend breaks. Kanso: weekly close below the 30-week SMA. 🔷 |
| **Risk per trade** | How much of your capital you lose if the stop is hit. |
| **Position sizing** | How much money goes into each trade. Kanso: equal slices, e.g. capital ÷ 10 positions. 🔷 |
| **Slots / max positions** | How many stocks you can hold at once. When more signals arrive than there are free slots, Kanso takes the ones with the highest RS first. 🔷 |
| **Liquidity** | How easily you can buy or sell without moving the price. Large caps are liquid, while tiny stocks may not be. |
| **Lot size** | The fixed quantity futures and options trade in, e.g. 1 lot = 250 shares. Not needed for stocks. |

## 7. Backtesting and performance

| Term | What it means |
|---|---|
| **Backtest** | Running a rule over past data to see how it *would have* performed. It's evidence, not a promise. 🔷 |
| **System / strategy** | A complete set of rules: what to buy, when to buy, when to sell, how much. Kanso's System Builder makes these. 🔷 |
| **Scanner / screener** | A tool that lists stocks matching rules *today*. A backtest asks the same question about the *past*. 🔷 |
| **Look-ahead bias** | Accidentally using information you couldn't have had at the time, e.g. buying at a close you only learn after the market shuts. It makes backtests look far better than reality, and Kanso has a test that checks it never happens. 🔷 |
| **Survivorship bias** | Testing only stocks that still exist today, which silently drops the companies that failed or were removed from the index. That flatters results, so Kanso shows a warning. 🔷 |
| **Overfitting / curve fitting** | Tweaking settings until the past looks perfect. The "best" settings are then often just luck. Kanso counts how many variations you've tried. 🔷 |
| **In-sample / out-of-sample** | The data you tuned on vs fresh data you didn't touch. Good results out-of-sample are much more convincing. |
| **Equity curve** | A chart of your account value over time. 🔷 |
| **Benchmark** | What you compare against, usually buy-and-hold Nifty 50. If you can't beat it, just buy the index. 🔷 |
| **Buy and hold** | Buy once and never sell. The simplest benchmark. 🔷 |
| **Return / % gain** | (Sell price − buy price) ÷ buy price, after costs. 🔷 |
| **CAGR** | Compound Annual Growth Rate: the steady yearly return that would turn your start value into your end value. 🔷 |
| **Drawdown / max drawdown (MDD)** | The fall from a peak to a later low. The max drawdown is the worst such fall, and it measures how painful the ride was. −25% means your account was at one point 25% below its best. 🔷 |
| **Win rate** | % of trades that made money. Breakout systems often win **under half** the time, yet still profit because the winners are much bigger than the losers. 🔷 |
| **Average win / average loss** | Typical % gain on winners vs typical % loss on losers. 🔷 |
| **Expectancy** | Average result per trade, winners and losers combined. Positive means the system makes money on average. 🔷 |
| **Exposure** | % of the time money was actually invested in stocks. 🔷 |
| **Holding period** | How long a trade lasted. 🔷 |
| **Sharpe ratio** | Return per unit of volatility. Higher means a smoother ride for the return. |
| **Profit factor** | Total gains ÷ total losses. Above 1 = profitable. |
| **Oracle / benchmark test (Kanso)** | Running the same rule in vectorbt and checking that Kanso gets the same trades. A correctness check, not a performance one. 🔷 |

## 8. Alternative chart types (V2)

| Term | What it means |
|---|---|
| **Renko** | Price drawn as equal-sized bricks. A new brick appears only when the price moves by the brick size, so time and small moves are ignored. |
| **Brick size** | How far the price must move to draw a new Renko brick (fixed ₹ or ATR-based). |
| **Point & Figure (P&F)** | Columns of X's (rising) and O's (falling) on a grid. A new column starts only after a reversal of several boxes. It shows support and resistance very clearly. |
| **Box size / reversal** | P&F settings: how big each X or O is, and how many boxes against the trend start a new column (often 3). |
| **Double top buy (P&F)** | An X column rising one box above the previous X column's top: P&F's version of a breakout. |
| **Kagi / Line break** | Other price-only chart styles RZone supports. Not planned for Kanso. |

## 9. Derivatives (not in V1, but you'll hear these)

| Term | What it means |
|---|---|
| **F&O** | Futures & Options, India's derivatives market. |
| **Futures** | A contract to buy or sell at a set price on a future date. It's leveraged: small price moves mean big gains or losses. |
| **Options: call / put** | A call is the right (not obligation) to buy at a set price; a put is the right to sell. |
| **Expiry** | When a futures or options contract ends. Indian stock F&O expires monthly. |
| **Rollover** | Closing an expiring contract and opening the next month's one. |
| **Margin** | Money the broker blocks as a deposit to hold a leveraged position. |
| **Leverage** | Controlling more money than you put in. It multiplies both gains and losses. |
| **IV (implied volatility)** | The volatility the options market expects. A big driver of option prices. |
| **Greeks** | Delta, gamma, theta, vega: measures of how an option's price reacts to changes in price, time and volatility. |

## 10. Tools mentioned in Kanso

| Tool | What it is |
|---|---|
| **TradingView** | A popular charting website. **Pine Script** is its language for indicators and strategies, and the **Strategy Tester** backtests one stock at a time. |
| **Chartink** | A free Indian stock screener with a scan builder. |
| **PKScreener** | An open-source NSE screener with 40+ built-in scans (runs via Docker). |
| **RZone / TradePoint** | Definedge's research and charting tools: the inspiration for Kanso. |
| **vectorbt** | A fast Python backtesting library. Kanso uses it as an independent correctness check. |
| **eod2 / stock-pattern** | An open-source NSE data downloader and a chart-pattern detector. |
| **yfinance / Yahoo Finance** | A free (unofficial) source of historical prices. Kanso V1's data source. |
