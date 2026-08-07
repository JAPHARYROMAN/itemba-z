export function hasEveryPermission(permissions: readonly string[], required: readonly string[]): boolean {
  const granted = new Set(permissions);
  return required.every((permission) => granted.has(permission));
}

export function canReadSupplierApprovals(permissions: readonly string[]): boolean {
  return hasEveryPermission(permissions, ["masterdata.read", "purchases.sourcing.read"]);
}
