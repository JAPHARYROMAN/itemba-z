"use client";

import { useState } from "react";
import { useLanguage } from "@/components/language-provider";
import { LiveBadge } from "@/components/live-sales/live-state";
import { text } from "@/lib/i18n";
import type { InventoryControlWorkspace, InventoryPolicy, InventoryPolicyStatus, LotRegistration, PublicProblem } from "@/live-api/types";

async function send(body: unknown) {
	const response = await fetch("/api/live/inventory-control", { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": crypto.randomUUID() }, body: JSON.stringify(body) });
	const payload = await response.json();
	if (!response.ok) throw payload;
	return payload;
}

export function InventoryControlWorkbench({ workspace }: { workspace: InventoryControlWorkspace }) {
	const { l, locale } = useLanguage();
	const [inventory, setInventory] = useState(workspace.inventory);
	const [productId, setProductId] = useState(workspace.products[0]?.id ?? "");
	const [supplierId, setSupplierId] = useState("");
	const [receiptId, setReceiptId] = useState(workspace.receipts[0]?.id ?? "");
	const [lotNumber, setLotNumber] = useState("");
	const [quantity, setQuantity] = useState(1);
	const [expiresAt, setExpiresAt] = useState("");
	const [lotControlled, setLotControlled] = useState(true);
	const [reason, setReason] = useState("Govern inventory planning and traceability controls");
	const [busy, setBusy] = useState(false);
	const [problem, setProblem] = useState<PublicProblem | null>(null);
	const productNames = new Map(workspace.products.map((product) => [product.id, `${product.code} · ${product.name}`]));

	async function run<T>(work: () => Promise<T>, apply: (value: T) => void) {
		setBusy(true); setProblem(null);
		try { apply(await work()); } catch (error) { setProblem(error as PublicProblem); } finally { setBusy(false); }
	}
	function createPolicy(event: React.FormEvent) {
		event.preventDefault();
		void run(() => send({ action: "create_policy", command: { product_id: productId, cost_method: "MOVING_AVERAGE", lot_controlled: lotControlled, reorder_point: 20, reorder_quantity: 30, maximum_stock: 100, safety_stock: 10, lead_time_days: 7, preferred_supplier_id: supplierId || undefined, reason } }) as Promise<InventoryPolicy>, (value) => setInventory((current) => ({ ...current, policies: [value, ...current.policies] })));
	}
	function transitionPolicy(item: InventoryPolicy, status: InventoryPolicyStatus) {
		void run(() => send({ action: "transition_policy", id: item.id, status, reason: `Authorized ${status.toLowerCase()} inventory policy decision` }) as Promise<InventoryPolicy>, (value) => setInventory((current) => ({ ...current, policies: current.policies.map((policy) => policy.id === value.id ? { ...policy, ...value } : policy) })));
	}
	function registerLots(event: React.FormEvent) {
		event.preventDefault();
		void run(() => send({ action: "register_lots", receiptId, command: { reason, lines: [{ product_id: productId, lot_number: lotNumber, quantity, expires_at: expiresAt ? new Date(expiresAt).toISOString() : undefined }] } }) as Promise<LotRegistration>, (value) => setInventory((current) => ({ ...current, registrations: [value, ...current.registrations] })));
	}
	const expiring = inventory.lots.filter((lot) => lot.quantity > 0 && lot.expires_at).toSorted((a, b) => String(a.expires_at).localeCompare(String(b.expires_at)));
	return <section className="page-stack">
		<header className="module-header"><div><p className="eyebrow">{l(text("Inventory intelligence", "Akili ya hesabu ya bidhaa"))}</p><h1>{l(text("Replenishment, lots & costing", "Ujazaji, mafungu na gharama"))}</h1><p>{l(text("Approved warehouse policies drive reorder recommendations, FEFO traceability, and retained moving-average evidence.", "Sera zilizoidhinishwa huongoza mapendekezo ya kujaza, ufuatiliaji wa FEFO na ushahidi wa wastani wa gharama."))}</p></div><LiveBadge context={workspace.context}/></header>
		<section className="metric-grid"><article className="metric-card"><span>{l(text("Reorder actions", "Hatua za kujaza"))}</span><strong>{inventory.replenishments.filter((item) => item.action_required).length}</strong></article><article className="metric-card"><span>{l(text("Live lots", "Mafungu hai"))}</span><strong>{inventory.lots.filter((lot) => lot.quantity > 0).length}</strong></article><article className="metric-card"><span>{l(text("Awaiting approval", "Zinasubiri idhini"))}</span><strong>{inventory.policies.filter((policy) => policy.status === "SUBMITTED").length}</strong></article></section>
		{problem ? <div className="problem-banner" role="alert"><strong>{problem.code}</strong><span>{problem.detail}</span></div> : null}
		<div className="workspace-grid"><section className="card operation-compose"><div className="card-heading"><div><h2>{l(text("Warehouse policy", "Sera ya ghala"))}</h2><p>{l(text("A checker must activate each revision.", "Mkaguzi lazima aidhinishe kila toleo."))}</p></div></div><form className="operation-form" onSubmit={createPolicy}><label>{l(text("Product", "Bidhaa"))}<select required value={productId} onChange={(event) => setProductId(event.target.value)}>{workspace.products.map((product) => <option key={product.id} value={product.id}>{productNames.get(product.id)}</option>)}</select></label><label>{l(text("Preferred supplier", "Msambazaji anayependekezwa"))}<select value={supplierId} onChange={(event) => setSupplierId(event.target.value)}><option value="">—</option>{workspace.suppliers.filter((supplier) => supplier.active).map((supplier) => <option key={supplier.id} value={supplier.id}>{supplier.code} · {supplier.name}</option>)}</select></label><label><input type="checkbox" checked={lotControlled} onChange={(event) => setLotControlled(event.target.checked)}/>{l(text(" Require lot and expiry control", " Hitaji udhibiti wa fungu na muda"))}</label><label className="operation-reason">{l(text("Business reason", "Sababu ya biashara"))}<textarea required minLength={8} value={reason} onChange={(event) => setReason(event.target.value)}/></label><button className="primary-button" disabled={busy || !productId}>{l(text("Create policy revision", "Unda toleo la sera"))}</button></form></section>
		<section className="card operation-compose"><div className="card-heading"><div><h2>{l(text("Receipt lot allocation", "Ugawaji wa fungu la mapokezi"))}</h2><p>{l(text("Allocation must exactly equal the approved receipt before posting.", "Ugawaji lazima ulingane kabisa na mapokezi yaliyoidhinishwa kabla ya kutumwa."))}</p></div></div><form className="operation-form" onSubmit={registerLots}><label>{l(text("Approved receipt", "Mapokezi yaliyoidhinishwa"))}<select required value={receiptId} onChange={(event) => setReceiptId(event.target.value)}>{workspace.receipts.map((receipt) => <option key={receipt.id} value={receipt.id}>{receipt.number}</option>)}</select></label><label>{l(text("Lot number", "Namba ya fungu"))}<input required value={lotNumber} onChange={(event) => setLotNumber(event.target.value)}/></label><label>{l(text("Quantity", "Kiasi"))}<input required type="number" min="1" value={quantity} onChange={(event) => setQuantity(Number(event.target.value))}/></label><label>{l(text("Expiry", "Muda wa matumizi"))}<input type="datetime-local" value={expiresAt} onChange={(event) => setExpiresAt(event.target.value)}/></label><button className="secondary-button" disabled={busy || !receiptId || !productId}>{l(text("Register exact allocation", "Sajili ugawaji kamili"))}</button></form></section></div>
		<section className="card records-card"><div className="records-toolbar"><div><h2>{l(text("Replenishment recommendations", "Mapendekezo ya kujaza"))}</h2><span>{inventory.replenishments.length}</span></div></div><div className="table-scroll"><table className="data-table"><thead><tr><th>{l(text("Product", "Bidhaa"))}</th><th>{l(text("Available", "Inayopatikana"))}</th><th>{l(text("Incoming", "Inayokuja"))}</th><th>{l(text("Projected", "Makadirio"))}</th><th>{l(text("Recommend", "Pendekezo"))}</th></tr></thead><tbody>{inventory.replenishments.map((item) => <tr key={item.product_id}><td>{productNames.get(item.product_id) ?? item.product_id}</td><td>{item.available}</td><td>{item.incoming}</td><td>{item.projected}</td><td><strong>{item.recommended_quantity}</strong></td></tr>)}</tbody></table></div></section>
		<section className="card records-card"><div className="records-toolbar"><div><h2>{l(text("Policy approvals", "Idhini za sera"))}</h2><span>{inventory.policies.length}</span></div></div><div className="table-scroll"><table className="data-table"><thead><tr><th>{l(text("Product", "Bidhaa"))}</th><th>{l(text("Method", "Njia"))}</th><th>{l(text("Status", "Hali"))}</th><th>{l(text("Control", "Udhibiti"))}</th></tr></thead><tbody>{inventory.policies.map((item) => <tr key={item.id}><td>{productNames.get(item.product_id) ?? item.product_id}</td><td>{item.cost_method}{item.lot_controlled ? " · LOT" : ""}</td><td>{item.status}</td><td>{item.status === "DRAFT" ? <button className="secondary-button" disabled={busy} onClick={() => transitionPolicy(item, "SUBMITTED")}>{l(text("Submit", "Wasilisha"))}</button> : item.status === "SUBMITTED" ? <><button className="primary-button" disabled={busy} onClick={() => transitionPolicy(item, "ACTIVE")}>{l(text("Activate", "Washa"))}</button><button className="secondary-button" disabled={busy} onClick={() => transitionPolicy(item, "REJECTED")}>{l(text("Reject", "Kataa"))}</button></> : null}</td></tr>)}</tbody></table></div></section>
		<section className="workspace-grid"><article className="card records-card"><div className="card-heading"><h2>{l(text("Expiry watch", "Uangalizi wa muda"))}</h2></div><div className="table-scroll"><table><thead><tr><th>{l(text("Lot", "Fungu"))}</th><th>{l(text("Quantity", "Kiasi"))}</th><th>{l(text("Expires", "Muda"))}</th></tr></thead><tbody>{expiring.map((lot) => <tr key={lot.lot_id}><td>{lot.lot_number}<br/><small>{productNames.get(lot.product_id)}</small></td><td>{lot.quantity}</td><td>{new Date(String(lot.expires_at)).toLocaleDateString(locale)}</td></tr>)}</tbody></table></div></article><article className="card records-card"><div className="card-heading"><h2>{l(text("Cost evidence", "Ushahidi wa gharama"))}</h2></div><div className="table-scroll"><table><thead><tr><th>{l(text("Product", "Bidhaa"))}</th><th>{l(text("Before", "Kabla"))}</th><th>{l(text("Receipt", "Mapokezi"))}</th><th>{l(text("After", "Baada"))}</th></tr></thead><tbody>{inventory.cost_history.map((entry) => <tr key={entry.id}><td>{productNames.get(entry.product_id) ?? entry.product_id}</td><td>{entry.cost_before_minor.toLocaleString()}</td><td>{entry.receipt_cost_minor.toLocaleString()}</td><td><strong>{entry.cost_after_minor.toLocaleString()}</strong></td></tr>)}</tbody></table></div></article></section>
	</section>;
}
