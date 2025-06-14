-- Migration: create_products_table
-- Created: 2025-06-14 18:03:39
-- Batch: 1

-- +migrate Up
CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create indexes for performance
CREATE INDEX idx_products_created_at ON products(created_at);

-- +migrate Down
DROP INDEX IF EXISTS idx_products_created_at;
DROP TABLE IF EXISTS products;