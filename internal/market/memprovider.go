package market

import (
	"context"
	"fmt"
	"sort"
)

// MemProvider is an in-memory Provider. It is safe for concurrent reads.
type MemProvider struct {
	bySymbol  map[string]Instrument
	byID      map[int64]Instrument
	bars      map[string][]Bar
	universes map[string][]string
}

var _ Provider = (*MemProvider)(nil)

// NewMemProvider builds a provider over instruments, bars keyed by symbol
// (sorted ascending by date) and universes of symbols. Universes never have
// historical membership.
func NewMemProvider(insts []Instrument, bars map[string][]Bar, universes map[string][]string) *MemProvider {
	p := &MemProvider{
		bySymbol:  make(map[string]Instrument, len(insts)),
		byID:      make(map[int64]Instrument, len(insts)),
		bars:      bars,
		universes: universes,
	}
	for _, in := range insts {
		p.bySymbol[in.Symbol] = in
		p.byID[in.ID] = in
	}
	return p
}

// MemProviderFromBars builds a provider from bars keyed by symbol, as read
// by ReadCSV. Instruments are NSE with lot size 1, IDs assigned 1.. in
// symbol order; symbols starting with "^" are indices, the rest equities.
// One universe, "ALL", lists every equity.
func MemProviderFromBars(bars map[string][]Bar) *MemProvider {
	syms := make([]string, 0, len(bars))
	for s := range bars {
		syms = append(syms, s)
	}
	sort.Strings(syms)
	insts := make([]Instrument, 0, len(syms))
	var equities []string
	for i, s := range syms {
		typ := Equity
		if len(s) > 0 && s[0] == '^' {
			typ = Index
		} else {
			equities = append(equities, s)
		}
		insts = append(insts, Instrument{ID: int64(i + 1), Exchange: "NSE", Symbol: s, Type: typ, LotSize: 1})
	}
	return NewMemProvider(insts, bars, map[string][]string{"ALL": equities})
}

// Instrument implements Provider.
func (p *MemProvider) Instrument(_ context.Context, symbol string) (Instrument, error) {
	in, ok := p.bySymbol[symbol]
	if !ok {
		return Instrument{}, fmt.Errorf("instrument %q: %w", symbol, ErrNotFound)
	}
	return in, nil
}

// Daily implements Provider.
func (p *MemProvider) Daily(_ context.Context, id int64, from, to Date) ([]Bar, error) {
	in, ok := p.byID[id]
	if !ok {
		return nil, fmt.Errorf("instrument id %d: %w", id, ErrNotFound)
	}
	all := p.bars[in.Symbol]
	lo := sort.Search(len(all), func(i int) bool { return all[i].Date >= from })
	hi := sort.Search(len(all), func(i int) bool { return all[i].Date > to })
	if lo >= hi {
		return []Bar{}, nil
	}
	out := make([]Bar, hi-lo)
	copy(out, all[lo:hi])
	return out, nil
}

// Universe implements Provider.
func (p *MemProvider) Universe(_ context.Context, name string) ([]Instrument, bool, error) {
	syms, ok := p.universes[name]
	if !ok {
		return nil, false, fmt.Errorf("universe %q: %w", name, ErrNotFound)
	}
	out := make([]Instrument, 0, len(syms))
	for _, s := range syms {
		in, ok := p.bySymbol[s]
		if !ok {
			return nil, false, fmt.Errorf("universe %q member %q: %w", name, s, ErrNotFound)
		}
		out = append(out, in)
	}
	return out, false, nil
}

// Universes implements Provider.
func (p *MemProvider) Universes(_ context.Context) ([]string, error) {
	names := make([]string, 0, len(p.universes))
	for n := range p.universes {
		names = append(names, n)
	}
	sort.Strings(names)
	return names, nil
}
