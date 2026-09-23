# PostGIS

Laravel authoring, sqlc call site. The service imports `gen` only.

## Setup

Uncomment PostGIS in `migrations/extensions.sql` (this example already has the line enabled):

```sql
CREATE EXTENSION IF NOT EXISTS postgis;
```

```bash
vorm migrate
vorm generate
```

PostGIS is a Postgres extension, so the column uses the generic custom type rather than a portable Blueprint method:

```go
t.CustomType("GEOGRAPHY(POINT, 4326)").Column("location")
t.CustomType("GEOMETRY(POINT, 3857)").Column("shape")
```

`geography` and `geometry` scan as `string` (EWKB hex text). Inserts take EWKT, for example `SRID=4326;POINT(-73.9857 40.7484)`.

## Call the generated query

`queries/places.go` is the stub. `vorm generate` lowers it to a SQL const in `vorm/gen/places.sql.go` (`?` becomes `$1`, `$2`, `$3`). Application code calls that function:

```go
func (s *Service) Nearby(ctx context.Context, lon, lat float64, meters int) ([]gen.NearbyPlacesRow, error) {
	return gen.NearbyPlaces(ctx, s.db, gen.NearbyPlacesParams{Lon: lon, Lat: lat, Meters: meters})
}
```
