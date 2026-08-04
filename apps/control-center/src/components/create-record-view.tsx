"use client";

import Link from "next/link";
import { useState } from "react";
import { ArrowLeft, Check, CircleCheck, Info, Save } from "lucide-react";
import type { ModuleData } from "@/domain/erp";
import { useLanguage } from "@/components/language-provider";

export function CreateRecordView({ module }: { module: ModuleData }) {
  const { t, l } = useLanguage();
  const [validated, setValidated] = useState(false);

  return (
    <div className="page-stack create-page">
      <Link className="back-link" href={`/${module.key}`}><ArrowLeft size={16} />{t("cancel")} · {l(module.title)}</Link>
      <section className="page-heading"><div><p className="eyebrow">{l(module.eyebrow)}</p><h1>{l(module.primaryAction)}</h1><p>{t("createIntro")}</p></div></section>
      <div className="create-layout">
        <aside className="form-steps" aria-label={t("workflowSteps")}><div className="active"><span>1</span><strong>{t("details")}</strong><small>{l(module.singular)}</small></div><div><span>2</span><strong>{t("review")}</strong><small>{t("policiesApprovals")}</small></div><div><span>3</span><strong>{t("post")}</strong><small>{t("ledgerAuditEvent")}</small></div></aside>
        <form className="card record-form" onSubmit={(event) => { event.preventDefault(); setValidated(true); }}>
          <div className="form-notice"><Info size={18} /><p>{t("prototypeNotice")}</p></div>
          {validated ? <div className="validation-banner" role="status"><CircleCheck size={18} />{t("validationPassed")}</div> : null}
          <div className="form-grid">{module.formFields.map((field) => <label key={field.name} className={field.type === "textarea" ? "field-wide" : ""}><span>{l(field.label)}{field.required ? <em>{t("required")}</em> : null}</span>{field.type === "textarea" ? <textarea name={field.name} placeholder={l(field.placeholder)} rows={4} required={field.required} /> : field.type === "select" ? <select name={field.name} defaultValue="" required={field.required}><option value="" disabled>{l(field.placeholder)}</option>{field.options?.map((option) => <option key={option.en} value={option.en}>{l(option)}</option>)}</select> : <input name={field.name} type={field.type} placeholder={l(field.placeholder)} required={field.required} />}</label>)}</div>
          <div className="form-controls"><Link className="secondary-button" href={`/${module.key}`}>{t("cancel")}</Link><button className="secondary-button" type="button"><Save size={17} />{t("saveDraft")}</button><button className="primary-button" type="submit"><Check size={17} />{t("validateDraft")}</button></div>
        </form>
      </div>
    </div>
  );
}
