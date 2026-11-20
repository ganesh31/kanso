export function Footer() {
  return (
    <footer className="text-muted-foreground border-t px-4 py-4 text-center text-xs">
      Data: Yahoo Finance · Charts:{" "}
      <a className="underline" href="https://www.tradingview.com/" target="_blank" rel="noreferrer">
        TradingView Lightweight Charts
      </a>{" "}
      · non-commercial portfolio demo
    </footer>
  );
}
