"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { Boxes, CheckCircle2, LockKeyhole, ShieldAlert, ShieldCheck, Smartphone, TriangleAlert } from "lucide-react";
import { useState } from "react";
import { useLanguage } from "@/components/language-provider";
import { LiveBadge } from "@/components/live-sales/live-state";
import { compactId, formatTimestamp } from "@/live-api/format";
import {
  clearPendingCommandAfterSuccess, deviceAllocationPendingScope, deviceStatusPendingScope,
  markPendingCommandRejected, PendingCommandConflictError, reservePendingCommand,
} from "@/live-api/pending-command";
import type {
  ChangeMobileDeviceAllocationCommand, ChangeMobileDeviceStatusCommand,
  DeviceManagementWorkspace, MobileDevice, PublicProblem,
} from "@/live-api/types";
import { text } from "@/lib/i18n";

function isProblem(value: unknown): value is PublicProblem {
  return Boolean(value && typeof value === "object" && typeof (value as Record<string, unknown>).code === "string");
}

function isDevice(value: unknown): value is MobileDevice {
  return Boolean(value && typeof value === "object" && typeof (value as Record<string, unknown>).device_id === "string");
}

export function DeviceManagement({ workspace }: { workspace: DeviceManagementWorkspace }) {
  const { l } = useLanguage();
  const canManage = workspace.context.permissions.includes("mobile.devices.manage");
  return <div className="page-stack devices-page">
    <section className="page-heading module-heading"><div><div className="heading-badges"><LiveBadge context={workspace.context} /><span className="scope-chip">{workspace.context.company_name} · {workspace.context.branch_name}</span></div><h1>{l(text("POS device governance", "Usimamizi wa vifaa vya POS"))}</h1><p>{l(text("Suspend compromised devices and govern offline stock reservations from live warehouse evidence.", "Simamisha vifaa vilivyo hatarini na dhibiti akiba ya bidhaa nje ya mtandao kwa ushahidi hai wa ghala."))}</p></div></section>
    <aside className="live-control-note"><ShieldCheck size={18} /><div><strong>{l(text("Every change is reasoned and immutable", "Kila badiliko lina sababu na halibadiliki"))}</strong><p>{l(text("Suspension ends current offline authorization. Allocation changes cannot undercut synchronized use or overcommit warehouse stock.", "Kusimamisha kunamaliza ruhusa ya sasa nje ya mtandao. Mabadiliko ya mgao hayawezi kupunguza matumizi yaliyosawazishwa au kuzidisha bidhaa za ghala."))}</p></div></aside>
    {workspace.devices.length ? <div className="device-grid">{workspace.devices.map((device) => <DeviceCard key={device.device_id} device={device} workspace={workspace} canManage={canManage} />)}</div> : <section className="card live-empty"><CheckCircle2 size={30} /><strong>{l(text("No enrolled devices in this scope", "Hakuna vifaa vilivyosajiliwa katika upeo huu"))}</strong></section>}
    {workspace.nextCursor ? <Link className="secondary-button device-next" href={`/devices?cursor=${encodeURIComponent(workspace.nextCursor)}`}>{l(text("Next page", "Ukurasa unaofuata"))}</Link> : null}
  </div>;
}

function DeviceCard({ device, workspace, canManage }: { device: MobileDevice; workspace: DeviceManagementWorkspace; canManage: boolean }) {
  const { locale, l } = useLanguage();
  const router = useRouter();
  const [statusReason, setStatusReason] = useState("");
  const [productId, setProductId] = useState(workspace.products[0]?.id ?? "");
  const currentAllocation = device.stock_allocations.find((item) => item.product_id === productId)?.allocated_quantity ?? 0;
  const [quantity, setQuantity] = useState(String(currentAllocation));
  const [allocationReason, setAllocationReason] = useState("");
  const [confirmed, setConfirmed] = useState(false);
  const [submitting, setSubmitting] = useState<"status" | "allocation" | null>(null);
  const [problem, setProblem] = useState<PublicProblem | null>(null);
  const targetStatus: ChangeMobileDeviceStatusCommand["status"] = device.status === "ACTIVE" ? "SUSPENDED" : "ACTIVE";

  async function sendCommand<T extends object>(kind: "status" | "allocation", command: T, path: string, scope: string) {
    const fingerprint = JSON.stringify(command);
    let pending;
    try {
      pending = reservePendingCommand({ storage: window.sessionStorage, scope, fingerprint, payload: command });
    } catch (error) {
      setProblem({ type: "about:blank", title: "Command not sent", status: 409, code: "pending_device_command", detail: error instanceof PendingCommandConflictError ? l(text("A different device command still awaits an authoritative result.", "Amri tofauti ya kifaa bado inasubiri matokeo rasmi.")) : l(text("The command identity could not be preserved.", "Utambulisho wa amri haukuweza kuhifadhiwa.")) });
      return;
    }
    setSubmitting(kind);
    setProblem(null);
    try {
      const response = await fetch(path, { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": pending.record.key, Accept: "application/json" }, body: fingerprint });
      const payload: unknown = await response.json();
      if (!response.ok) {
        if (response.status >= 400 && response.status < 500 && response.status !== 409) {
          try { markPendingCommandRejected(window.sessionStorage, scope, pending.record.key, response.status); } catch { /* exact key remains available */ }
        }
        setProblem(isProblem(payload) ? payload : { type: "about:blank", title: "Command rejected", status: response.status, code: "device_command_rejected", detail: "The live ERP rejected the device command." });
        return;
      }
      if (!isDevice(payload)) throw new Error("Device response is invalid");
      try { clearPendingCommandAfterSuccess(window.sessionStorage, scope, pending.record.key); } catch { /* backend replay remains safe */ }
      setStatusReason(""); setAllocationReason(""); setConfirmed(false); router.refresh();
    } catch {
      setProblem({ type: "about:blank", title: "Connection interrupted", status: 503, code: "bff_unavailable", detail: l(text("Retry is safe with the preserved command identity.", "Kujaribu tena ni salama kwa utambulisho wa amri uliohifadhiwa.")) });
    } finally {
      setSubmitting(null);
    }
  }

  const parsedQuantity = Number(quantity);
  const allocationValid = Number.isSafeInteger(parsedQuantity) && parsedQuantity >= 0 && allocationReason.trim().length >= 8 && confirmed;
  return <article className="card device-card">
    <div className="device-card-heading"><span className="metric-icon"><Smartphone size={19} /></span><div><strong>{device.device_name}</strong><small>{compactId(device.device_id)} · {compactId(device.actor_id)}</small></div><span className={`reconciliation-status status-${device.status.toLowerCase()}`}>{device.status}</span></div>
    <dl className="live-context-details"><div><dt>{l(text("Last seen", "Ilionekana mwisho"))}</dt><dd>{formatTimestamp(device.last_seen_at, locale, workspace.context.timezone)}</dd></div><div><dt>{l(text("Installed publication", "Chapisho lililosakinishwa"))}</dt><dd>MD {device.master_data_version} · PRICE {device.price_version}</dd></div><div><dt>{l(text("Offline authorization", "Ruhusa nje ya mtandao"))}</dt><dd>{formatTimestamp(device.offline_sales_valid_until, locale, workspace.context.timezone)}</dd></div></dl>
    <div className="device-allocation-list"><strong><Boxes size={15} />{l(text("Offline allocations", "Mgao nje ya mtandao"))}</strong>{device.stock_allocations.length ? device.stock_allocations.map((item) => <span key={item.product_id}>{workspace.products.find((product) => product.id === item.product_id)?.name ?? compactId(item.product_id)}<small>{item.remaining_quantity} / {item.allocated_quantity}</small></span>) : <small>{l(text("No stock reserved", "Hakuna bidhaa iliyohifadhiwa"))}</small>}</div>
    {canManage && device.status !== "REVOKED" ? <div className="device-actions">
      <section><h3><ShieldAlert size={16} />{targetStatus === "SUSPENDED" ? l(text("Suspend device", "Simamisha kifaa")) : l(text("Reactivate device", "Rudisha kifaa"))}</h3><textarea rows={2} maxLength={500} value={statusReason} onChange={(event) => { setStatusReason(event.target.value); setConfirmed(false); }} placeholder={l(text("Verified reason (minimum 8 characters)", "Sababu iliyothibitishwa (angalau herufi 8)"))} /><button className={targetStatus === "SUSPENDED" ? "danger-button" : "secondary-button"} disabled={submitting !== null || statusReason.trim().length < 8 || !confirmed} onClick={() => sendCommand("status", { status: targetStatus, reason: statusReason.trim() } satisfies ChangeMobileDeviceStatusCommand, `/api/live/mobile/devices/${encodeURIComponent(device.device_id)}/status-changes`, deviceStatusPendingScope(workspace.context, device.device_id))}>{submitting === "status" ? l(text("Recording…", "Inarekodi…")) : targetStatus === "SUSPENDED" ? l(text("Suspend now", "Simamisha sasa")) : l(text("Reactivate", "Rudisha"))}</button></section>
      <section><h3><Boxes size={16} />{l(text("Change allocation", "Badilisha mgao"))}</h3><select value={productId} onChange={(event) => { const next = event.target.value; setProductId(next); setQuantity(String(device.stock_allocations.find((item) => item.product_id === next)?.allocated_quantity ?? 0)); setConfirmed(false); }}>{workspace.products.map((product) => <option key={product.id} value={product.id}>{product.name} · {product.available_quantity}</option>)}</select><input type="number" min="0" step="1" value={quantity} onChange={(event) => { setQuantity(event.target.value); setConfirmed(false); }} /><textarea rows={2} maxLength={500} value={allocationReason} onChange={(event) => { setAllocationReason(event.target.value); setConfirmed(false); }} placeholder={l(text("Verified allocation reason", "Sababu iliyothibitishwa ya mgao"))} /><button className="secondary-button" disabled={submitting !== null || !allocationValid || !productId} onClick={() => sendCommand("allocation", { product_id: productId, allocated_quantity: parsedQuantity, reason: allocationReason.trim() } satisfies ChangeMobileDeviceAllocationCommand, `/api/live/mobile/devices/${encodeURIComponent(device.device_id)}/allocation-changes`, deviceAllocationPendingScope(workspace.context, device.device_id))}>{submitting === "allocation" ? l(text("Recording…", "Inarekodi…")) : l(text("Record allocation", "Rekodi mgao"))}</button></section>
      <label className="confirmation-check"><input type="checkbox" checked={confirmed} onChange={(event) => setConfirmed(event.target.checked)} /><span>{l(text("I verified the device, reason, and live stock evidence.", "Nimethibitisha kifaa, sababu na ushahidi hai wa bidhaa."))}</span></label>
      {problem ? <div className="sale-problem" role="alert"><TriangleAlert size={17} /><span><strong>{problem.code}</strong>{problem.detail}</span></div> : null}
    </div> : <div className="permission-empty"><LockKeyhole size={22} /><strong>{device.status === "REVOKED" ? l(text("Revocation is permanent", "Kufuta ruhusa ni kwa kudumu")) : l(text("Management permission required", "Ruhusa ya usimamizi inahitajika"))}</strong></div>}
  </article>;
}
