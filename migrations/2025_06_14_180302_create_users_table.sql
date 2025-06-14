-- Migration: create_users_table
-- Created: 2025-06-14 18:03:02
-- Batch: 1

-- +migrate Up
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create indexes for performance
CREATE INDEX idx_users_created_at ON users(created_at);

-- +migrate Down
DROP INDEX IF EXISTS idx_users_created_at;
DROP TABLE IF EXISTS users;