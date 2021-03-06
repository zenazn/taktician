package ai

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/nelhage/taktician/bitboard"
	"github.com/nelhage/taktician/tak"
)

type Weights struct {
	TopFlat  int
	Standing int
	Capstone int

	Flat     int
	Captured int

	Liberties int

	Tempo int

	Groups [8]int
}

var DefaultWeights = Weights{
	TopFlat:  400,
	Standing: 200,
	Capstone: 300,

	Flat:      100,
	Liberties: 20,

	Captured: 25,

	Tempo: 250,

	Groups: [8]int{
		0,   // 0
		0,   // 1
		0,   // 2
		100, // 3
		300, // 4
		500, // 5
	},
}

func MakeEvaluator(w *Weights) EvaluationFunc {
	return func(m *MinimaxAI, p *tak.Position) int64 {
		return evaluate(w, m, p)
	}
}

var DefaultEvaluate = MakeEvaluator(&DefaultWeights)

func evaluate(w *Weights, m *MinimaxAI, p *tak.Position) int64 {
	if over, winner := p.GameOver(); over {
		var pieces int64
		if winner == tak.White {
			pieces = int64(p.WhiteStones())
		} else {
			pieces = int64(p.BlackStones())
		}
		switch winner {
		case tak.NoColor:
			return 0
		case p.ToMove():
			return maxEval - int64(p.MoveNumber()) + pieces
		default:
			return minEval + int64(p.MoveNumber()) - pieces
		}
	}

	var ws, bs int64

	if p.ToMove() == tak.White {
		ws += int64(w.Tempo)
	} else {
		bs += int64(w.Tempo)
	}
	analysis := p.Analysis()

	ws += int64(bitboard.Popcount(p.White&^p.Caps&^p.Standing) * w.TopFlat)
	bs += int64(bitboard.Popcount(p.Black&^p.Caps&^p.Standing) * w.TopFlat)
	ws += int64(bitboard.Popcount(p.White&p.Standing) * w.Standing)
	bs += int64(bitboard.Popcount(p.Black&p.Standing) * w.Standing)
	ws += int64(bitboard.Popcount(p.White&p.Caps) * w.Capstone)
	bs += int64(bitboard.Popcount(p.Black&p.Caps) * w.Capstone)

	for i, h := range p.Height {
		if h <= 1 {
			continue
		}
		s := p.Stacks[i] & ((1 << (h - 1)) - 1)
		bf := bitboard.Popcount(s)
		wf := int(h) - bf - 1
		ws += int64(wf * w.Flat)
		bs += int64(bf * w.Flat)
		captured := int(h - 1)
		if captured > p.Size()-1 {
			captured = p.Size() - 1
		}
		if p.White&(1<<uint(i)) != 0 {
			ws += int64(captured * w.Captured)
		} else {
			bs += int64(captured * w.Captured)
		}
	}

	ws += int64(m.scoreGroups(analysis.WhiteGroups, w))
	bs += int64(m.scoreGroups(analysis.BlackGroups, w))

	wr := p.White &^ p.Standing
	br := p.Black &^ p.Standing
	wl := bitboard.Popcount(bitboard.Grow(&m.c, ^p.Black, wr) &^ wr)
	bl := bitboard.Popcount(bitboard.Grow(&m.c, ^p.White, br) &^ br)
	ws += int64(w.Liberties * wl)
	bs += int64(w.Liberties * bl)

	if p.ToMove() == tak.White {
		return ws - bs
	}
	return bs - ws
}

func (ai *MinimaxAI) scoreGroups(gs []uint64, ws *Weights) int {
	sc := 0
	for _, g := range gs {
		w, h := bitboard.Dimensions(&ai.c, g)

		sc += ws.Groups[w]
		sc += ws.Groups[h]
	}

	return sc
}

func ExplainScore(m *MinimaxAI, out io.Writer, p *tak.Position) {
	tw := tabwriter.NewWriter(out, 4, 8, 1, '\t', 0)
	fmt.Fprintf(tw, "\twhite\tblack\n")
	var scores [2]struct {
		flats    int
		standing int
		caps     int

		stones   int
		captured int
	}

	scores[0].flats = bitboard.Popcount(p.White &^ p.Caps &^ p.Standing)
	scores[1].flats = bitboard.Popcount(p.Black &^ p.Caps &^ p.Standing)
	scores[0].standing = bitboard.Popcount(p.White & p.Standing)
	scores[1].standing = bitboard.Popcount(p.Black & p.Standing)
	scores[0].caps = bitboard.Popcount(p.White & p.Caps)
	scores[1].caps = bitboard.Popcount(p.Black & p.Caps)

	for i, h := range p.Height {
		if h <= 1 {
			continue
		}
		s := p.Stacks[i] & ((1 << (h - 1)) - 1)
		bf := bitboard.Popcount(s)
		wf := int(h) - bf - 1
		scores[0].stones += wf
		scores[1].stones += bf

		captured := int(h - 1)
		if captured > p.Size()-1 {
			captured = p.Size() - 1
		}
		if p.White&(1<<uint(i)) != 0 {
			scores[0].captured += captured
		} else {
			scores[1].captured += captured
		}
	}

	fmt.Fprintf(tw, "flats\t%d\t%d\n", scores[0].flats, scores[1].flats)
	fmt.Fprintf(tw, "standing\t%d\t%d\n", scores[0].standing, scores[1].standing)
	fmt.Fprintf(tw, "caps\t%d\t%d\n", scores[0].caps, scores[1].caps)
	fmt.Fprintf(tw, "captured\t%d\t%d\n", scores[0].captured, scores[1].captured)
	fmt.Fprintf(tw, "stones\t%d\t%d\n", scores[0].stones, scores[1].stones)

	analysis := p.Analysis()

	wr := p.White &^ p.Standing
	br := p.Black &^ p.Standing
	wl := bitboard.Popcount(bitboard.Grow(&m.c, ^p.Black, wr) &^ wr)
	bl := bitboard.Popcount(bitboard.Grow(&m.c, ^p.White, br) &^ br)

	fmt.Fprintf(tw, "liberties\t%d\t%d\n", wl, bl)

	for i, g := range analysis.WhiteGroups {
		w, h := bitboard.Dimensions(&m.c, g)
		fmt.Fprintf(tw, "g%d\t%dx%x\n", i, w, h)
	}
	for i, g := range analysis.BlackGroups {
		w, h := bitboard.Dimensions(&m.c, g)
		fmt.Fprintf(tw, "g%d\t\t%dx%x\n", i, w, h)
	}
	tw.Flush()
}
