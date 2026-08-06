SET search_path TO itembaz, public;
ALTER TABLE report_exports DROP COLUMN sha256;
ALTER TABLE report_exports DROP COLUMN export_format;
