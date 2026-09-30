package generate

import "testing"

func TestJSONTag(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"id", "id"},
		{"display_name", "display_name"},
		{"DisplayName", "display_name"},
		{"UserID", "user_id"},
		{"posts_count", "posts_count"},
		{"CreatedAt", "created_at"},
	}
	for _, tc := range cases {
		if got := JSONTag(tc.in); got != tc.want {
			t.Errorf("JSONTag(%q)=%q, want %q", tc.in, got, tc.want)
		}
	}
}
