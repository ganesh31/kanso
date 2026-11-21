// Package report is the Stock Report (recogniser): every registered
// pattern evaluated on one instrument at one date, with its evidence.
package report

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"kanso/internal/market"
	"kanso/internal/patterns"
)

// Line is one condition's verdict in a report.
type Line struct {
	Type   string          `json:"type"`
	Label  string          `json:"label"`
	Params patterns.Params `json:"params"`
	Result patterns.Result `json:"result"`
}

// Report is the Stock Report for one symbol.
type Report struct {
	Symbol string `json:"symbol"`
	// AsOf is the date of the bar evaluated, taken from the data (never
	// today's calendar); empty when there is no bar on or before the
	// requested date.
	AsOf      market.Date      `json:"as_of"`
	Timeframe market.Timeframe `json:"timeframe"`
	// Level and LevelDate are the shared breakout level at AsOf, if any.
	Level     *float64     `json:"level"`
	LevelDate *market.Date `json:"level_date"`
	Lines     []Line       `json:"lines"`
}

// Run evaluates every registered pattern with default params on symbol at
// the last bar on or before asOf (the latest bar when asOf is empty).
func Run(ctx context.Context, prov market.Provider, symbol string, asOf market.Date, tf market.Timeframe) (*Report, error) {
	inst, err := prov.Instrument(ctx, symbol)
	if err != nil {
		return nil, err
	}
	to := asOf
	if to == "" {
		to = "9999-12-31"
	}
	bench, err := loadBenchmark(ctx, prov, to)
	if err != nil {
		return nil, err
	}
	pctx, _, err := patterns.Load(ctx, prov, inst, bench, tf, "", to)
	if err != nil {
		return nil, err
	}
	s := pctx.Series
	t := s.Complete - 1

	rep := &Report{Symbol: symbol, Timeframe: tf, Lines: []Line{}}
	if t >= 0 {
		rep.AsOf = s.Bars[t].Date
	}
	reg := patterns.Default()
	for _, m := range reportOrder(reg.All()) {
		pat, _ := reg.Get(m.Type)
		params, err := patterns.Normalize(m, nil)
		if err != nil {
			return nil, fmt.Errorf("default params of %s: %w", m.Type, err)
		}
		if err := pat.Prepare(pctx, params); err != nil {
			return nil, fmt.Errorf("prepare %s: %w", m.Type, err)
		}
		res := patterns.NotEvaluableResult(patterns.ReasonInsufficientHistory)
		if t >= 0 {
			res = pat.Eval(pctx, params, t)
		}
		rep.Lines = append(rep.Lines, Line{Type: m.Type, Label: m.Label, Params: params, Result: res})
	}
	if t >= 0 && t < len(pctx.LevelIdx) && pctx.LevelIdx[t] >= 0 {
		level, levelDate := pctx.Level[t], s.Bars[pctx.LevelIdx[t]].Date
		rep.Level, rep.LevelDate = &level, &levelDate
	}
	return rep, nil
}

// loadBenchmark loads the default benchmark's daily bars up to to (the
// report runs default params, so relative strength uses it). A missing
// benchmark is not an error: relative strength reports benchmark_missing
// and every other condition still evaluates.
func loadBenchmark(ctx context.Context, prov market.Provider, to market.Date) (*market.Series, error) {
	inst, err := prov.Instrument(ctx, patterns.DefaultBenchmark)
	if errors.Is(err, market.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("benchmark %s: %w", patterns.DefaultBenchmark, err)
	}
	bars, err := prov.Daily(ctx, inst.ID, "", to)
	if err != nil {
		return nil, fmt.Errorf("benchmark %s: %w", patterns.DefaultBenchmark, err)
	}
	return market.NewDaily(inst, bars), nil
}

// reportOrder puts context providers (n_year_high, which provides the
// breakout level) first so requirers can read it; the rest follow by type.
func reportOrder(metas []patterns.Meta) []patterns.Meta {
	sort.SliceStable(metas, func(i, j int) bool {
		pi, pj := len(metas[i].Provides) > 0, len(metas[j].Provides) > 0
		if pi != pj {
			return pi
		}
		return metas[i].Type < metas[j].Type
	})
	return metas
}
