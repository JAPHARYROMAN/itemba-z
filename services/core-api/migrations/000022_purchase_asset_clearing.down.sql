SET search_path TO itembaz, public;
DROP INDEX IF EXISTS fixed_assets_purchase_source_unique;
ALTER TABLE fixed_assets DROP CONSTRAINT IF EXISTS fixed_assets_source_product_fk, DROP CONSTRAINT IF EXISTS fixed_assets_source_document_fk, DROP COLUMN IF EXISTS source_product_id, DROP COLUMN IF EXISTS source_document_id;
