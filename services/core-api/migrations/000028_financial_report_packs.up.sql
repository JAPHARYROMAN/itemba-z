SET search_path TO itembaz, public;
ALTER TABLE report_exports ADD COLUMN export_format text NOT NULL DEFAULT 'CSV' CHECK(export_format IN('CSV','PDF','XLSX'));
ALTER TABLE report_exports ADD COLUMN sha256 char(64) NOT NULL DEFAULT repeat('0',64);
