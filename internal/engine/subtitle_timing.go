package engine

import (
	"math"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Timings are approximate, using the decoded voice duration and word lengths.
// They use the game clock so pausing does not advance the reveal.
type subtitleTiming struct {
	text            string
	start, duration float64
	ends            []int
	starts          []float64
}

func newSubtitleTiming(text string, start, duration float64) *subtitleTiming {
	t := &subtitleTiming{text: text, start: start, duration: duration}
	weight := 0.0
	inWord, begin := false, 0
	endWord := func(end int) {
		t.ends = append(t.ends, end)
		t.starts = append(t.starts, weight)
		word := text[begin:end]
		weight += float64(max(2, min(10, utf8.RuneCountInString(word))))
		if strings.ContainsAny(word, ".!?;:") {
			weight += 4
		} else if strings.Contains(word, ",") {
			weight += 2
		}
	}
	for i, r := range text {
		if unicode.IsSpace(r) {
			if inWord {
				endWord(i)
				inWord = false
			}
		} else if !inWord {
			begin, inWord = i, true
		}
	}
	if inWord {
		endWord(len(text))
	}
	for i := range t.starts {
		t.starts[i] *= duration / weight
	}
	return t
}

// Return byte offsets for the fully visible prefix and the fading word.
// Keeping the complete subtitle for layout prevents wrapping from jumping.
func (c *PuppetController) subtitleReveal() (int, int, float64) {
	p := c.Puppet
	if p == nil {
		return 0, 0, 1
	}
	t := p.subtitleTiming
	if !c.TimedSubtitles || t == nil || t.text != p.Subtitle || len(t.ends) == 0 {
		return len(p.Subtitle), len(p.Subtitle), 1
	}
	elapsed := max(0, c.Executor.Now()-t.start)
	if elapsed >= t.duration {
		return len(p.Subtitle), len(p.Subtitle), 1
	}
	i := 0
	for i+1 < len(t.starts) && elapsed >= t.starts[i+1] {
		i++
	}
	end := t.duration
	if i+1 < len(t.starts) {
		end = t.starts[i+1]
	}
	fade := max(1, min(120, end-t.starts[i]))
	alpha := min(1, .15+.85*math.Floor((elapsed-t.starts[i])/fade*6)/6)
	from := 0
	if i > 0 {
		from = t.ends[i-1]
	}
	return from, t.ends[i], alpha
}
