package service

import (
	"context"

	"github.com/vorzela/vorm/examples/postgis/vorm/gen"
	"github.com/vorzela/vorm/query"
)

// Service is the app layer. It calls generated functions the way a sqlc caller does.
type Service struct {
	db query.DB
}

// New builds a service over a pool opened with query.OpenPostgres.
func New(db query.DB) *Service { return &Service{db: db} }

// Nearby returns places within meters of a WGS84 point.
func (s *Service) Nearby(ctx context.Context, lon, lat float64, meters int) ([]gen.NearbyPlacesRow, error) {
	return gen.NearbyPlaces(ctx, s.db, gen.NearbyPlacesParams{Lon: lon, Lat: lat, Meters: meters})
}
