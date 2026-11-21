import { createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute("/")({
  component: Home,
});

function Home() {
  return (
    <div className="flex flex-col gap-2">
      <h1 className="text-2xl font-semibold tracking-tight">Multi-year breakouts, honestly tested</h1>
      <p className="text-muted-foreground">
        Enter a symbol above to open its Stock Report: every pattern it passes or fails today, with the evidence.
      </p>
    </div>
  );
}
