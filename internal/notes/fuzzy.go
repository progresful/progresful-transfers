package notes

const (
	// maxFuzzyLen bounds the word length the fuzzy matcher looks at; longer
	// tokens are never keywords.
	maxFuzzyLen = 32
	// minTypoLen is the shortest keyword that dealers mistype. In the notes
	// seen so far only words of six letters or more carry typos, so a shorter
	// keyword is never the target of a fuzzy match: "there" is not "three".
	minTypoLen = 6
)

// fuzzyBudget is how many edits a typed word of the given length may be away
// from a keyword. A word under five letters cannot be a mistyped keyword of
// six, so it must match exactly.
func fuzzyBudget(n int) int {
	switch {
	case n < minTypoLen-1:
		return 0
	case n < 9:
		return 1
	default:
		return 2
	}
}

// osaDistance is the optimal-string-alignment distance (Levenshtein plus
// adjacent transposition) between a and b, or limit+1 when it exceeds limit.
// Dealer typos are almost always one swapped, doubled or dropped letter, which
// this metric counts as a single edit.
func osaDistance(a, b string, limit int) int {
	la, lb := len(a), len(b)
	if la > maxFuzzyLen || lb > maxFuzzyLen {
		return limit + 1
	}
	if d := la - lb; d > limit || -d > limit {
		return limit + 1
	}
	var rows [3][maxFuzzyLen + 1]int
	prev2, prev, cur := &rows[0], &rows[1], &rows[2]
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		cur[0] = i
		best := cur[0]
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			v := prev[j-1] + cost
			if d := prev[j] + 1; d < v {
				v = d
			}
			if d := cur[j-1] + 1; d < v {
				v = d
			}
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				if d := prev2[j-2] + 1; d < v {
					v = d
				}
			}
			cur[j] = v
			if v < best {
				best = v
			}
		}
		if best > limit {
			return limit + 1
		}
		prev2, prev, cur = prev, cur, prev2
	}
	if prev[lb] > limit {
		return limit + 1
	}
	return prev[lb]
}
