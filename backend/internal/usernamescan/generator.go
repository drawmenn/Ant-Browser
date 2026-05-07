package usernamescan

import (
	"sort"
	"strings"
)

const fallbackCharPool = "aeiourstlnmcd"

var commonFillers = []string{
	"x", "y", "z", "r", "n", "s", "o", "a",
	"io", "ly", "go", "on", "er", "it", "me", "up", "ai", "ox",
	"lab", "hub", "box", "cat", "fox", "zen", "jet", "max", "pro", "sky", "bit",
	"nova", "byte", "zone", "nest", "link", "wave", "mint", "grid", "flow",
	"base", "spark", "shift", "scope", "prime",
}

var digitFillers = []string{
	"1", "7", "8", "9", "01", "07", "08", "11", "21", "88", "99", "007", "101", "365", "777", "888",
}

type rankedCandidate struct {
	name  string
	score int
}

func GenerateCandidates(options GeneratorOptions) []string {
	targetLength := options.TargetLength
	if targetLength <= 0 {
		targetLength = 6
	}
	targetLength = clampInt(targetLength, 6, 30)

	maxResults := options.MaxResults
	if maxResults <= 0 {
		maxResults = 80
	}
	maxResults = clampInt(maxResults, 10, 500)

	sourceTerms := extractTerms(options.SourceText)
	if len(sourceTerms) == 0 {
		return []string{}
	}

	baseFragments := buildBaseFragments(sourceTerms, targetLength)
	charPool := buildCharPool(sourceTerms, options.AllowDigits)
	fillCache := make(map[int][]string)
	candidates := make(map[string]int)

	remember := func(raw string, bonus int) {
		candidate := normalizeCandidate(raw)
		if len(candidate) != targetLength {
			return
		}
		score := scoreCandidate(candidate, sourceTerms) + bonus
		if existing, ok := candidates[candidate]; !ok || score > existing {
			candidates[candidate] = score
		}
	}

	for _, fragment := range baseFragments {
		if len(fragment) >= targetLength {
			remember(fragment[:targetLength], 26)
			remember(fragment[len(fragment)-targetLength:], 18)
			continue
		}

		remainder := targetLength - len(fragment)
		fillers := getFillers(remainder, charPool, options.AllowDigits, fillCache)
		for _, suffix := range fillers {
			remember(fragment+suffix, 20)
		}
		for _, prefix := range fillers {
			remember(prefix+fragment, 12)
		}

		if remainder >= 2 {
			leftSize := min(2, remainder-1)
			leftFillers := getFillers(leftSize, charPool, options.AllowDigits, fillCache)
			rightFillers := getFillers(remainder-leftSize, charPool, options.AllowDigits, fillCache)
			for _, left := range firstN(leftFillers, 12) {
				for _, right := range firstN(rightFillers, 12) {
					remember(left+fragment+right, 10)
				}
			}
		}
	}

	if len(sourceTerms) >= 2 {
		combined := strings.Join(sourceTerms, "")
		remember(combined[:min(len(combined), targetLength)], 30)
		initials := ""
		for _, term := range sourceTerms {
			if term != "" {
				initials += term[:1]
			}
		}
		if initials != "" {
			remainder := targetLength - len(initials)
			for _, filler := range getFillers(remainder, charPool, options.AllowDigits, fillCache) {
				remember(initials+filler, 24)
				remember(filler+initials, 14)
			}
		}
	}

	ordered := make([]rankedCandidate, 0, len(candidates))
	for name, score := range candidates {
		ordered = append(ordered, rankedCandidate{name: name, score: score})
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].score == ordered[j].score {
			return ordered[i].name < ordered[j].name
		}
		return ordered[i].score > ordered[j].score
	})

	results := make([]string, 0, min(maxResults, len(ordered)))
	for _, item := range firstNRanked(ordered, maxResults) {
		results = append(results, item.name)
	}
	return results
}

func extractTerms(sourceText string) []string {
	var b strings.Builder
	for _, r := range strings.ToLower(sourceText) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		b.WriteByte(' ')
	}

	seen := make(map[string]struct{})
	var terms []string
	for _, token := range strings.Fields(b.String()) {
		if token == "" {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		terms = append(terms, token)
	}
	return terms
}

func buildBaseFragments(sourceTerms []string, targetLength int) []string {
	var fragments []string
	push := func(value string) {
		cleaned := normalizeCandidate(value)
		if len(cleaned) < 2 || containsString(fragments, cleaned) {
			return
		}
		fragments = append(fragments, cleaned)
	}

	combined := strings.Join(sourceTerms, "")
	if combined != "" {
		push(combined[:min(len(combined), targetLength)])
	}

	for _, term := range sourceTerms {
		push(term)
		push(term[:min(len(term), targetLength)])
		if len(term) >= 3 {
			push(term[:3])
			push(term[len(term)-3:])
		}
		if len(term) >= 4 {
			push(term[:4])
			push(term[len(term)-4:])
		}

		if len(term) > 1 {
			var compact strings.Builder
			compact.WriteByte(term[0])
			for i := 1; i < len(term); i++ {
				if !strings.ContainsRune("aeiou", rune(term[i])) {
					compact.WriteByte(term[i])
				}
			}
			if compact.Len() >= 3 {
				push(compact.String())
			}
		}
	}

	if len(sourceTerms) >= 2 {
		first := sourceTerms[0]
		second := sourceTerms[1]
		push(first + second)
		push(first[:min(3, len(first))] + second[:min(3, len(second))])
		push(first[:1] + second)
		push(first + second[:1])
		initials := ""
		for _, term := range sourceTerms {
			if term != "" {
				initials += term[:1]
			}
		}
		push(initials)
	}
	return fragments
}

func buildCharPool(sourceTerms []string, allowDigits bool) []byte {
	seen := make(map[byte]struct{})
	var pool []byte
	source := strings.Join(sourceTerms, "") + fallbackCharPool
	for i := 0; i < len(source); i++ {
		ch := source[i]
		if ch >= '0' && ch <= '9' && !allowDigits {
			continue
		}
		if _, ok := seen[ch]; ok {
			continue
		}
		seen[ch] = struct{}{}
		pool = append(pool, ch)
	}
	if len(pool) == 0 {
		return []byte(fallbackCharPool)
	}
	return pool
}

func getFillers(length int, charPool []byte, allowDigits bool, cache map[int][]string) []string {
	if cached, ok := cache[length]; ok {
		return cached
	}
	if length <= 0 {
		cache[length] = []string{""}
		return cache[length]
	}

	tokens := append([]string{}, commonFillers...)
	if allowDigits {
		tokens = append(tokens, digitFillers...)
	}
	for _, ch := range firstNBytes(charPool, 8) {
		token := string(ch)
		if !containsString(tokens, token) {
			tokens = append(tokens, token)
		}
	}
	for _, left := range firstNBytes(charPool, 5) {
		for _, right := range firstNBytes(charPool, 5) {
			token := string([]byte{left, right})
			if !containsString(tokens, token) {
				tokens = append(tokens, token)
			}
		}
	}

	var results []string
	seen := make(map[string]struct{})
	add := func(value string) {
		if len(value) != length {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		results = append(results, value)
	}

	for _, token := range tokens {
		add(token)
	}

	if length <= 3 {
		var walk func(prefix string, depth int)
		letters := firstNBytes(charPool, 6)
		walk = func(prefix string, depth int) {
			if len(results) >= 80 {
				return
			}
			if depth == length {
				add(prefix)
				return
			}
			for _, ch := range letters {
				walk(prefix+string(ch), depth+1)
			}
		}
		walk("", 0)
	}

	if length > 1 && len(results) < 120 {
		for split := 1; split < length && len(results) < 120; split++ {
			leftItems := getFillers(split, charPool, allowDigits, cache)
			rightItems := getFillers(length-split, charPool, allowDigits, cache)
			for _, left := range firstN(leftItems, 12) {
				for _, right := range firstN(rightItems, 12) {
					add(left + right)
					if len(results) >= 120 {
						break
					}
				}
				if len(results) >= 120 {
					break
				}
			}
		}
	}

	if len(results) == 0 {
		results = []string{strings.Repeat("x", length)}
	}
	cache[length] = results
	return results
}

func normalizeCandidate(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func scoreCandidate(candidate string, sourceTerms []string) int {
	score := 100
	digitCount := 0
	vowelCount := 0
	for _, ch := range candidate {
		if ch >= '0' && ch <= '9' {
			digitCount++
		}
		if strings.ContainsRune("aeiou", ch) {
			vowelCount++
		}
	}
	score -= digitCount * 7

	for _, term := range sourceTerms {
		if strings.Contains(candidate, term) {
			score += min(20, len(term)*3)
		} else if strings.HasPrefix(candidate, term[:min(3, len(term))]) {
			score += 9
		}
	}

	if candidate[0] >= '0' && candidate[0] <= '9' {
		score -= 20
	}
	if hasTripleRepeat(candidate) {
		score -= 16
	}
	if hasThreeTrailingDigits(candidate) {
		score -= 8
	}

	consonantCount := len(candidate) - digitCount - vowelCount
	if vowelCount == 0 || consonantCount == 0 {
		score -= 10
	} else if absInt(vowelCount-consonantCount) <= 2 {
		score += 8
	}
	if isSortedAscending(candidate) {
		score -= 4
	}
	return score
}

func hasTripleRepeat(value string) bool {
	for i := 2; i < len(value); i++ {
		if value[i] == value[i-1] && value[i] == value[i-2] {
			return true
		}
	}
	return false
}

func hasThreeTrailingDigits(value string) bool {
	if len(value) < 3 {
		return false
	}
	for _, ch := range value[len(value)-3:] {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func isSortedAscending(value string) bool {
	for i := 1; i < len(value); i++ {
		if value[i] < value[i-1] {
			return false
		}
	}
	return true
}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func firstN(items []string, count int) []string {
	if len(items) <= count {
		return items
	}
	return items[:count]
}

func firstNBytes(items []byte, count int) []byte {
	if len(items) <= count {
		return items
	}
	return items[:count]
}

func firstNRanked(items []rankedCandidate, count int) []rankedCandidate {
	if len(items) <= count {
		return items
	}
	return items[:count]
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
