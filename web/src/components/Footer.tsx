export function Footer() {
  return (
    <footer className="text-muted-foreground border-t">
      <p className="mx-auto max-w-6xl px-4 py-4 text-xs">
        Prices from Yahoo Finance, charts by{" "}
        <a
          className="underline underline-offset-2"
          href="https://www.tradingview.com/"
          target="_blank"
          rel="noreferrer"
        >
          TradingView Lightweight Charts
        </a>
        . A non-commercial portfolio project; nothing here is investment advice.
      </p>
    </footer>
  );
}
