package query

// Col is a typed SQL column name for write maps (Create / Update / Upsert / …).
// Prefer generated keys such as models.Users.Col.Email over free-form strings.
// Untyped string constants still work in map literals: map[Col]any{"email": v}.
type Col string

// Values is a column→value map for inserts and updates.
type Values = map[Col]any

// String returns the underlying column name.
func (c Col) String() string { return string(c) }

// stringMap converts typed column maps for internal SQL builders.
func stringMap(values map[Col]any) map[string]any {
	if values == nil {
		return nil
	}
	out := make(map[string]any, len(values))
	for k, v := range values {
		out[string(k)] = v
	}
	return out
}

// stringMaps converts a slice of typed column maps.
func stringMaps(rows []map[Col]any) []map[string]any {
	if rows == nil {
		return nil
	}
	out := make([]map[string]any, len(rows))
	for i, row := range rows {
		out[i] = stringMap(row)
	}
	return out
}
