SET search_path TO itembaz, public;
DROP TABLE IF EXISTS fixed_asset_depreciation,fixed_asset_transitions,fixed_assets,budget_transitions,budget_lines,budgets;
DROP FUNCTION IF EXISTS protect_advanced_finance();
DELETE FROM role_permissions WHERE permission_code IN('finance.budgets.read','finance.budgets.manage','finance.budgets.approve','finance.assets.read','finance.assets.manage','finance.assets.approve','finance.assets.depreciate','finance.assets.dispose');
DELETE FROM permissions WHERE code IN('finance.budgets.read','finance.budgets.manage','finance.budgets.approve','finance.assets.read','finance.assets.manage','finance.assets.approve','finance.assets.depreciate','finance.assets.dispose');
