-- Enable PostGIS before the places migration. `vorm migrate` runs this file
-- first on PostgreSQL. Other extensions stay commented until you need them.
CREATE EXTENSION IF NOT EXISTS postgis;
