import {
  createColumnHelper,
  createSortedRowModel,
  rowSortingFeature,
  type SortingState,
  sortFn_alphanumeric,
  tableFeatures,
  useTable,
} from "@tanstack/react-table";
import type { Match } from "@/api/client";
import { formatAgeShort, formatPrice, formatSignedPct } from "@/lib/format";

const features = tableFeatures({
  rowSortingFeature,
  sortedRowModel: createSortedRowModel(),
  sortFns: { alphanumeric: sortFn_alphanumeric },
});
const helper = createColumnHelper<typeof features, Match>();

const dash = <span className="text-muted-foreground">–</span>;

/** Numeric columns: a missing value (condition absent or not evaluable) always sorts last. */
const numeric = { sortUndefined: "last" as const, sortDescFirst: false };

const baseColumns = [
  helper.accessor("symbol", { header: "Symbol", sortFn: "alphanumeric" }),
  helper.accessor("level", {
    header: "Level",
    ...numeric,
    cell: (c) => {
      const v = c.getValue();
      return v === undefined ? dash : formatPrice(v);
    },
  }),
  helper.accessor("level_date", {
    header: "Level date",
    sortUndefined: "last",
    cell: (c) => c.getValue() ?? dash,
  }),
  helper.accessor((m) => m.level_age_bars, {
    id: "level_age",
    header: "Old high age",
    ...numeric,
    cell: (c) => {
      const m = c.row.original;
      return m.level_date ? formatAgeShort(m.level_date, m.date) : dash;
    },
  }),
  helper.accessor("pct_above", {
    header: "% above",
    ...numeric,
    cell: (c) => {
      const v = c.getValue();
      return v === undefined ? dash : `${v.toFixed(2)}%`;
    },
  }),
  helper.accessor("vol_ratio", {
    header: "Volume ×",
    ...numeric,
    cell: (c) => {
      const v = c.getValue();
      return v === undefined ? dash : `${v.toFixed(2)}×`;
    },
  }),
  helper.accessor("rs_diff", {
    header: "RS vs Nifty",
    ...numeric,
    cell: (c) => {
      const v = c.getValue();
      return v === undefined ? dash : formatSignedPct(v * 100);
    },
  }),
  helper.accessor((m) => m.signal_date ?? m.true_since, {
    id: "since",
    header: "Signal / true since",
    sortUndefined: "last",
    cell: (c) => c.getValue() ?? dash,
  }),
];

const provisionalColumn = helper.accessor((m) => (m.provisional ? (m.provisional.pass ? 1 : 0) : undefined), {
  id: "provisional",
  header: "This week",
  ...numeric,
  cell: (c) => {
    const p = c.row.original.provisional;
    if (!p) return dash;
    return p.evaluable ? `${p.pass ? "✅" : "❌"} provisional` : "not evaluable";
  },
});

// Stable column sets (module scope, as Table v9 asks); the weekly one adds the week in progress.
const dailyColumns = helper.columns([...baseColumns]);
const weeklyColumns = helper.columns([...baseColumns, provisionalColumn]);

/** "pct_above" ↔ [{id: "pct_above", desc: false}], "-pct_above" ↔ desc. */
export function parseSort(sort: string | undefined): SortingState {
  const s = sort || "pct_above";
  return s.startsWith("-") ? [{ id: s.slice(1), desc: true }] : [{ id: s, desc: false }];
}

function formatSort(state: SortingState): string | undefined {
  const first = state[0];
  if (!first) return undefined;
  return first.desc ? `-${first.id}` : first.id;
}

/**
 * Scan matches, sortable by any column (one column at a time). Sorting is stable, so ties keep the
 * server's order (by % above, then symbol), and missing values always sort last.
 */
export function MatchTable({
  matches,
  sort,
  onSortChange,
  onOpen,
}: {
  matches: Match[];
  sort?: string;
  onSortChange: (sort: string | undefined) => void;
  onOpen: (symbol: string) => void;
}) {
  const withProvisional = matches.some((m) => m.provisional !== undefined);
  const cols = withProvisional ? weeklyColumns : dailyColumns;
  const sorting = parseSort(sort);
  const table = useTable({
    features,
    columns: cols,
    data: matches,
    state: { sorting },
    enableSortingRemoval: false,
    onSortingChange: (updater) => {
      onSortChange(formatSort(typeof updater === "function" ? updater(sorting) : updater));
    },
  });

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          {table.getHeaderGroups().map((group) => (
            <tr key={group.id} className="border-b text-left">
              {group.headers.map((header) => {
                const dir = header.column.getIsSorted();
                return (
                  <th
                    key={header.id}
                    className="px-2 py-2 font-medium whitespace-nowrap"
                    aria-sort={dir === "asc" ? "ascending" : dir === "desc" ? "descending" : "none"}
                  >
                    <button
                      type="button"
                      className="hover:text-foreground inline-flex items-center gap-1"
                      onClick={header.column.getToggleSortingHandler()}
                    >
                      <table.FlexRender header={header} />
                      <span aria-hidden className="text-muted-foreground">
                        {dir === "asc" ? "▲" : dir === "desc" ? "▼" : ""}
                      </span>
                    </button>
                  </th>
                );
              })}
            </tr>
          ))}
        </thead>
        <tbody>
          {table.getRowModel().rows.map((row) => (
            <tr
              key={row.id}
              className="hover:bg-accent/50 cursor-pointer border-b"
              onClick={() => onOpen(row.original.symbol)}
            >
              {row.getAllCells().map((cell) =>
                cell.column.id === "symbol" ? (
                  <td key={cell.id} className="px-2 py-1.5 font-medium">
                    <button
                      type="button"
                      className="hover:underline"
                      onClick={(e) => {
                        e.stopPropagation();
                        onOpen(row.original.symbol);
                      }}
                    >
                      {row.original.symbol}
                    </button>
                  </td>
                ) : (
                  <td key={cell.id} className="px-2 py-1.5 tabular-nums whitespace-nowrap">
                    <table.FlexRender cell={cell} />
                  </td>
                ),
              )}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
