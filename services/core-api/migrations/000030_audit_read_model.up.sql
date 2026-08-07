SET search_path TO itembaz, public;

INSERT INTO permissions(code, description) VALUES
  ('audit.read', 'View immutable audit timelines in the authorized legal-company scope')
ON CONFLICT (code) DO NOTHING;
