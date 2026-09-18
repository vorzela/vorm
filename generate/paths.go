package generate

// Default layout (app root) — no sqlc:
//
//	migrations/         — numbered Blueprint Go (vorm make migration)
//	queries/            — // vorm:query stubs (hand-written)
//	models/             — models from the live database (vorm generate models)
//	vorm/gen/           — generated Go (pgx v5 / MySQL drivers via query.DB)
const (
	DefaultSchemaDir     = "./migrations"
	DefaultQueryDir      = "./queries"
	DefaultModelDir      = "./models"
	DefaultOutDir        = "./vorm/gen"
	DefaultModelPkg      = "models"
	DefaultMigrationPath = "./migrations"
)
