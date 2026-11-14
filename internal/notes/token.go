package notes

import (
	"strings"
	"unicode"
)

type tokKind uint8

const (
	kWord tokKind = iota
	kNum
	kDash
)

// maxRefDigits is the longest digit run that can still be a clock time
// ("1400"); longer runs are reference numbers.
const maxRefDigits = 4

type token struct {
	kind   tokKind
	text   string // folded word
	f      feat
	val    int  // lexicon value (day offset, weekday, hour number, am/pm)
	num    int  // digits before the separator, as written
	min    int  // minutes after "." or ":", or None
	digits int  // how many digits num was written with
	suffix bool // "9:sta", "20:een": a number with a Finnish case ending
}

// segment is one clause: the words between two punctuation marks.
type segment struct {
	toks     []token
	sentence int  // segments of one sentence share the number
	corr     bool // begins with a correction marker ("eikun", "Korjaus:", "i mean")
}

// abbreviations end with a full stop that does not end the sentence.
var abbreviations = map[string]bool{
	"viim": true, "klo": true, "kl": true, "nro": true, "nr": true, "huom": true, "esim": true, "puh": true, "tel": true,
	"ma": true, "ti": true, "ke": true, "to": true, "pe": true, "la": true, "su": true,
	"man": true, "tis": true, "ons": true, "tors": true, "tor": true, "fre": true, "lor": true, "son": true,
	"mon": true, "tue": true, "tues": true, "wed": true, "thu": true, "thur": true, "thurs": true, "fri": true, "sat": true, "sun": true,
}

// meridiemDots rewrites "3 p.m." so that the full stops do not end the sentence.
var meridiemDots = strings.NewReplacer("a.m.", "am ", "p.m.", "pm ", "a.m", "am", "p.m", "pm")

// fold lowers the text and maps accented letters and typographic dashes to
// plain ASCII, so "Öppet", "oppet" and "ÖPPET" read the same.
func fold(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		r = unicode.ToLower(r)
		switch r {
		case 'ä', 'å', 'à', 'á', 'â', 'ã':
			r = 'a'
		case 'ö', 'ø', 'ó', 'ò', 'ô', 'õ':
			r = 'o'
		case 'é', 'è', 'ê', 'ë':
			r = 'e'
		case 'ü', 'ú', 'ù':
			r = 'u'
		case '–', '—', '−', '‐', '‑':
			r = '-'
		case '’', '‘', '`', '´':
			r = '\''
		}
		if r < 128 {
			b.WriteByte(byte(r))
		} else if unicode.IsLetter(r) {
			b.WriteByte('x') // unknown letter: keeps the word in one piece
		} else {
			b.WriteByte(' ')
		}
	}
	return b.String()
}

func isLetter(c byte) bool { return c >= 'a' && c <= 'z' }
func isDigit(c byte) bool  { return c >= '0' && c <= '9' }

type tokenizer struct {
	segs     []segment
	cur      []token
	sentence int
	corr     bool
}

// flush closes the current segment. hard also closes the sentence.
func (t *tokenizer) flush(hard bool) {
	if len(t.cur) > 0 {
		t.segs = append(t.segs, segment{toks: t.cur, sentence: t.sentence, corr: t.corr})
		t.cur = nil
		t.corr = false
	}
	if hard {
		// A marker alone before a comma ("sorry, 15:30") still corrects with
		// the next clause; it does not reach into the next sentence.
		t.corr = false
		t.sentence++
	}
}

// word adds a word. A correction marker starts a new segment in the same
// sentence, so that what follows it can replace what came before.
func (t *tokenizer) word(w string) {
	f, val := lookup(w)
	if n := len(t.cur); n > 0 && (w == "kun" || w == "ku" || w == "vaan") && t.cur[n-1].kind == kWord && t.cur[n-1].text == "ei" {
		// "ei kun" and "ei vaan" are the two-word spelling of "eikun".
		t.cur = t.cur[:n-1]
		t.flush(false)
		t.corr = true
		return
	}
	if f&fCorr != 0 {
		t.flush(false)
		t.corr = true
		return
	}
	if t.corr && len(t.cur) == 0 && (w == "i" || w == "jag") {
		return // "sorry, i mean ..."
	}
	t.cur = append(t.cur, token{kind: kWord, text: w, f: f, val: val, min: None})
}

// tokenize splits a note into clauses of classified words and numbers.
func tokenize(text string) []segment {
	s := meridiemDots.Replace(fold(text))
	t := &tokenizer{}
	n := len(s)
	for i := 0; i < n; {
		c := s[i]
		switch {
		case isLetter(c):
			j := i
			for j < n && (isLetter(s[j]) || (s[j] == '\'' && j+1 < n && isLetter(s[j+1]))) {
				j++
			}
			w := strings.ReplaceAll(s[i:j], "'", "")
			if j < n && s[j] == '.' && abbreviations[w] {
				j++
			}
			t.word(w)
			i = j
		case isDigit(c):
			var tok token
			tok, i = scanNumber(s, i)
			t.cur = append(t.cur, tok)
		case c == '-':
			if len(t.cur) > 0 {
				t.cur = append(t.cur, token{kind: kDash, min: None})
			}
			i++
		case c == '.' || c == '!' || c == '?' || c == ';' || c == '/' || c == '\n':
			t.flush(true)
			i++
		case c == ',' || c == '(' || c == ')':
			t.flush(false)
			i++
		default:
			i++
		}
	}
	t.flush(true)
	return t.segs
}

// scanNumber reads "15", "15.30", "8:15", "1400" or "9:sta" starting at s[i].
func scanNumber(s string, i int) (token, int) {
	n := len(s)
	j := i
	for j < n && isDigit(s[j]) {
		j++
	}
	tok := token{kind: kNum, min: None, digits: j - i}
	if tok.digits <= maxRefDigits {
		for k := i; k < j; k++ {
			tok.num = tok.num*10 + int(s[k]-'0')
		}
	}
	if tok.digits <= 2 && j+2 < n && (s[j] == '.' || s[j] == ':') && isDigit(s[j+1]) && isDigit(s[j+2]) && (j+3 >= n || !isDigit(s[j+3])) {
		tok.min = int(s[j+1]-'0')*10 + int(s[j+2]-'0')
		j += 3
	}
	if j+1 < n && s[j] == ':' && isLetter(s[j+1]) {
		j++
		for j < n && isLetter(s[j]) {
			j++
		}
		tok.suffix = true
	}
	return tok, j
}
