import Link from "next/link";
export default function NotFound() { return <div className="error-state"><span>404</span><h1>Record not found / Rekodi haijapatikana</h1><p>The requested workspace item does not exist or is outside your access scope.</p><Link className="primary-button" href="/">Return to dashboard</Link></div>; }
