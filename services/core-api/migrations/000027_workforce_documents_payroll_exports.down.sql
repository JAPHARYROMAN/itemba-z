SET search_path TO itembaz, public;
DROP FUNCTION IF EXISTS protect_workforce_status() CASCADE;
DROP TABLE IF EXISTS workforce_transitions,payroll_export_artifacts,employee_documents,shift_assignments,shift_templates CASCADE;
DELETE FROM role_permissions WHERE permission_code IN('hr.workforce.read','hr.shifts.manage','hr.shifts.approve','hr.documents.read','hr.documents.manage','hr.payroll.export');
DELETE FROM permissions WHERE code IN('hr.workforce.read','hr.shifts.manage','hr.shifts.approve','hr.documents.read','hr.documents.manage','hr.payroll.export');
