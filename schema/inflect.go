package schema

import "strings"

// Singularize is a small English heuristic for table → FK prefixes (posts → post).
func Singularize(word string) string {
	w := strings.ToLower(strings.TrimSpace(word))
	if w == "" {
		return w
	}
	switch {
	case strings.HasSuffix(w, "ies") && len(w) > 3:
		return w[:len(w)-3] + "y"
	case strings.HasSuffix(w, "ses") || strings.HasSuffix(w, "xes") || strings.HasSuffix(w, "zes") || strings.HasSuffix(w, "ches") || strings.HasSuffix(w, "shes"):
		return w[:len(w)-2]
	case strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss") && len(w) > 1:
		return w[:len(w)-1]
	}
	return w
}

// Pluralize is the inverse heuristic (post → posts).
func Pluralize(word string) string {
	w := strings.ToLower(strings.TrimSpace(word))
	if w == "" {
		return w
	}
	if strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss") {
		return w
	}
	switch {
	case strings.HasSuffix(w, "s") || strings.HasSuffix(w, "x") || strings.HasSuffix(w, "z") || strings.HasSuffix(w, "ch") || strings.HasSuffix(w, "sh"):
		return w + "es"
	case strings.HasSuffix(w, "y") && len(w) > 1 && !isVowel(rune(w[len(w)-2])):
		return w[:len(w)-1] + "ies"
	}
	return w + "s"
}

func isVowel(r rune) bool {
	return r == 'a' || r == 'e' || r == 'i' || r == 'o' || r == 'u'
}

// PivotName is Laravel's alphabetical join of the two singular table names.
func PivotName(leftTable, rightTable string) string {
	a, b := Singularize(leftTable), Singularize(rightTable)
	if a > b {
		a, b = b, a
	}
	return a + "_" + b
}
