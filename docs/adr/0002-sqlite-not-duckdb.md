# SQLite (WAL, pure Go) is the store; Parquet is only an export format

The engine loads each instrument's bars into Go and computes there. It never runs analytics in SQL, so DuckDB's columnar strengths went unused, while its costs did not: cgo builds, and a file lock that stops a second process (the ingest job) from opening the database. We use SQLite in WAL mode through the pure-Go `modernc.org/sqlite` driver. It is the single source of truth for bars, systems, universes and the ingest log. Parquet (`parquet-go`) is written only as an output, for the vectorbt oracle fixture (read with `pandas.read_parquet`) and for data exports. It is never read back as a source.

## Considered Options

- **DuckDB** (original spec): rejected for the reasons above. Revisit if V2 moves aggregation into SQL, such as the breakouts-per-day chart over 5,000 instruments.
- **Postgres**: expected in product stacks, but it needs a server to run locally and to host for the demo, and a 2.5M-row dataset doesn't need one.
- **Parquet as the bar store**: Parquet files are immutable, so every nightly update and every split re-fetch would rewrite whole files, and the data would live in two stores.
