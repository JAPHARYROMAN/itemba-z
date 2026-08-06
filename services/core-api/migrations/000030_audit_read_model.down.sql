SET search_path TO itembaz, public;

DELETE FROM role_permissions WHERE permission_code = 'audit.read';
DELETE FROM permissions WHERE code = 'audit.read';
