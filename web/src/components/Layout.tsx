import { Link, useNavigate } from "@tanstack/react-router";
import { useState, type FormEvent, type ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Footer } from "./Footer";

function SymbolSearch() {
  const navigate = useNavigate();
  const [symbol, setSymbol] = useState("");
  const submit = (e: FormEvent) => {
    e.preventDefault();
    const s = symbol.trim().toUpperCase();
    if (s) void navigate({ to: "/report/$symbol", params: { symbol: s } });
  };
  return (
    <form onSubmit={submit} className="flex w-full gap-2 sm:w-auto" role="search">
      <Input
        aria-label="Symbol"
        placeholder="NSE symbol, e.g. TCS"
        autoCapitalize="characters"
        autoCorrect="off"
        spellCheck={false}
        value={symbol}
        onChange={(e) => setSymbol(e.target.value)}
        className="flex-1 sm:w-48 sm:flex-none"
      />
      <Button type="submit" variant="outline">
        Report
      </Button>
    </form>
  );
}

function NavLink({ to, children }: { to: "/scanner" | "/builder"; children: ReactNode }) {
  return (
    <Link
      to={to}
      className="text-muted-foreground hover:text-foreground -mb-px border-b-2 border-transparent py-1 text-sm font-medium"
      activeProps={{ className: "!text-foreground !border-foreground" }}
    >
      {children}
    </Link>
  );
}

export function Layout({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-screen flex-col">
      <header className="border-b">
        <div className="mx-auto flex w-full max-w-6xl flex-wrap items-center justify-between gap-x-6 gap-y-3 px-4 py-3">
          <nav aria-label="Main" className="flex items-center gap-5">
            <Link to="/" className="text-xl font-bold tracking-tight">
              Kanso
            </Link>
            <NavLink to="/scanner">Scanner</NavLink>
            <NavLink to="/builder">Builder</NavLink>
          </nav>
          <SymbolSearch />
        </div>
      </header>
      <main className="mx-auto w-full max-w-6xl flex-1 px-4 py-8">{children}</main>
      <Footer />
    </div>
  );
}
