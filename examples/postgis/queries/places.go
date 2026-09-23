package queries

import (
	"context"

	"github.com/vorzela/vorm/examples/postgis/models"
	"github.com/vorzela/vorm/query"
)

// vorm:query name=NearbyPlaces
func NearbyPlaces(ctx context.Context, db query.DB, lon, lat float64, meters int) ([]models.Place, error) {
	return models.Places.WhereRaw(
		"ST_DWithin(location, ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography, ?)",
		lon, lat, meters,
	).OrderBy("id").Get(ctx, db)
}
