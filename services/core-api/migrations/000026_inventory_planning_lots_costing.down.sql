SET search_path TO itembaz, public;
DROP FUNCTION IF EXISTS protect_inventory_policy() CASCADE;
DROP TABLE IF EXISTS inventory_cost_history,inventory_lot_ledger,inventory_lots,inventory_lot_registration_lines,inventory_lot_registrations,inventory_policy_transitions,inventory_policies CASCADE;
DELETE FROM role_permissions WHERE permission_code IN('inventory.planning.read','inventory.planning.manage','inventory.planning.approve','inventory.lots.manage');
DELETE FROM permissions WHERE code IN('inventory.planning.read','inventory.planning.manage','inventory.planning.approve','inventory.lots.manage');
