SET search_path TO itembaz, public;

DELETE FROM role_permissions WHERE permission_code = 'dashboard.read';
DELETE FROM permissions WHERE code = 'dashboard.read';
