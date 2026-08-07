SET search_path TO itembaz, public;

REVOKE ALL ON catalog_publication_tax_rules FROM itembaz_runtime;
REVOKE ALL ON catalog_publication_products FROM itembaz_runtime;
REVOKE ALL ON catalog_publication_customers FROM itembaz_runtime;
REVOKE ALL ON catalog_publications FROM itembaz_runtime;

DROP TABLE catalog_publication_tax_rules;
DROP TABLE catalog_publication_products;
DROP TABLE catalog_publication_customers;
DROP TABLE catalog_publications;
