CREATE TABLE IF NOT EXISTS subscription_types (
    id UUID PRIMARY KEY,
    name VARCHAR(120) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_subscription_types_active
    ON subscription_types (created_at, id)
    WHERE deleted_at IS NULL;

INSERT INTO subscription_types (id, name)
VALUES
    ('8b7d8f9f-3b0a-4c89-a4d4-452ce5d763e1', 'Subscription Package'),
    ('be4e1df3-8f1e-4817-9815-94e37d4ef894', 'Additional Features'),
    ('02e58b8c-6420-4424-88e7-248db7f99079', 'E-Seal Document')
ON CONFLICT (id) DO UPDATE
SET
    name = EXCLUDED.name,
    updated_at = NOW(),
    deleted_at = NULL;
