ALTER TABLE account_usages ADD COLUMN warned_80_percent BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE account_usages ADD COLUMN warned_100_percent BOOLEAN NOT NULL DEFAULT false;
