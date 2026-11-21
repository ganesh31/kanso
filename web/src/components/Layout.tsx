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
    <form onSubmit={submit} className="flex gap-2" role="search">
      <Input
        aria-label="Symbol"
        placeholder="Symbol, e.g. TCS"
        value={symbol}
        onChange={(e) => setSymbol(e.target.value)}
        className="w-44"
      />
      <Button type="submit" variant="outline">
        Report
      </Button>
    </form>
  );
}

export function Layout({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-screen flex-col">
      <header className="flex items-center justify-between gap-4 border-b px-4 py-3">
        <Link to="/" className="text-lg font-semibold tracking-tight">
          Kanso
        </Link>
        <SymbolSearch />
      </header>
      <main className="mx-auto w-full max-w-5xl flex-1 px-4 py-6">{children}</main>
      <Footer />
    </div>
  );
}
