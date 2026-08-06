import Link from "next/link";
import { LockKeyhole, ShieldCheck } from "lucide-react";

interface LoginPageProps {
  searchParams: Promise<{ error?: string; signedOut?: string }>;
}

export default async function LoginPage({ searchParams }: LoginPageProps) {
  const parameters = await searchParams;
  const isDevelopment = process.env.ITEMBA_ENV === "development" && !process.env.ITEMBA_OIDC_ISSUER_URL?.trim();
  const message = parameters.signedOut === "1"
    ? "You have been signed out locally and provider revocation was requested. Umetoka salama na ubatilishaji wa mtoa huduma umeombwa."
    : parameters.error
      ? "The secure sign-in attempt could not be completed. Jaribio la kuingia salama halikukamilika; jaribu tena au wasiliana na msimamizi wa utambulisho."
      : "Use your organization identity to enter the ITEMBA-Z Control Center. Tumia utambulisho wa shirika lako kuingia kwenye Control Center.";

  return (
    <main className="login-page">
      <section className="login-card">
        <span className="login-mark"><LockKeyhole size={28} /></span>
        <p className="eyebrow">ITEMBA-Z · Control Center</p>
        <h1>Secure enterprise access <small>Ufikiaji salama wa biashara</small></h1>
        <p>{message}</p>
        {isDevelopment ? (
          <Link className="primary-button" href="/">Continue locally · Endelea ndani</Link>
        ) : (
          <Link className="primary-button" href="/api/auth/login">Sign in securely · Ingia kwa usalama</Link>
        )}
        <div className="login-assurance"><ShieldCheck size={17} /><span>Authorization code + PKCE · HttpOnly session · server-validated ERP scope</span></div>
      </section>
    </main>
  );
}
