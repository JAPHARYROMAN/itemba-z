SET search_path TO itembaz, public;
DROP TABLE IF EXISTS people_transitions,payroll_lines,payroll_runs,employee_loans,leave_requests,leave_types,attendance_entries,employees;
DROP FUNCTION IF EXISTS protect_people_master();
DELETE FROM role_permissions WHERE permission_code LIKE 'hr.%'; DELETE FROM permissions WHERE code LIKE 'hr.%';
