SET search_path TO itembaz, public;

INSERT INTO permissions(code, description) VALUES
  ('dashboard.read', 'View governed executive metrics and scoped transactional approval counts')
ON CONFLICT (code) DO NOTHING;
