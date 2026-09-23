//go:build ignore

package migrations

import "github.com/vorzela/vorm/schema"

func Up(s *schema.Facade) {
	s.Create("places", func(t *schema.Blueprint) {
		t.ID()
		t.String("name")
		t.CustomType("GEOGRAPHY(POINT, 4326)").Column("location")
		t.Timestamps()
		t.Raw(
			"CREATE INDEX IF NOT EXISTS places_location_gix ON places USING GIST (location);",
			"DROP INDEX IF EXISTS places_location_gix;",
		)
	})
}

func Down(s *schema.Facade) {
	s.DropIfExists("places")
}
