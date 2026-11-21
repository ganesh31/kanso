// Package backtest turns an entry rule into signals and trades. So far it
// holds signal detection, which the scanner shares.
package backtest

import (
	"kanso/internal/patterns"
	"kanso/internal/rules"
)

// Signals returns the bar indexes t where the entry rule fires (spec §5.5):
// Eval(t).Pass && Eval(t−1).Evaluable && !Eval(t−1).Pass. A not-evaluable
// t−1 (a data gap, too little history) never signals, so neither does the
// first evaluable bar, and bar 0 never does. Only complete bars
// (t < Complete) are evaluated. c must be prepared on ctx.
func Signals(c *rules.Compiled, ctx *patterns.Context) []int {
	out := []int{}
	n := ctx.Series.Complete
	if n < 2 {
		return out
	}
	prev := c.Eval(ctx, 0)
	for t := 1; t < n; t++ {
		cur := c.Eval(ctx, t)
		if cur.Pass && prev.Evaluable && !prev.Pass {
			out = append(out, t)
		}
		prev = cur
	}
	return out
}
