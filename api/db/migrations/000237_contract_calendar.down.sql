DROP INDEX IF EXISTS idx_public_contracts_expiry;
ALTER TABLE public_contracts
    DROP COLUMN IF EXISTS ata_pncp_id,
    DROP COLUMN IF EXISTS from_adhesion;
