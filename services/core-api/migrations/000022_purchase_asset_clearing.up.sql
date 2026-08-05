SET search_path TO itembaz, public;
ALTER TABLE fixed_assets ADD COLUMN source_document_id uuid, ADD COLUMN source_product_id uuid;
ALTER TABLE fixed_assets ADD CONSTRAINT fixed_assets_source_document_fk FOREIGN KEY(tenant_id,company_id,source_document_id) REFERENCES operation_documents(tenant_id,company_id,id);
ALTER TABLE fixed_assets ADD CONSTRAINT fixed_assets_source_product_fk FOREIGN KEY(tenant_id,company_id,source_product_id) REFERENCES products(tenant_id,company_id,id);
CREATE UNIQUE INDEX fixed_assets_purchase_source_unique ON fixed_assets(tenant_id,company_id,source_document_id,source_product_id) WHERE source_document_id IS NOT NULL;
