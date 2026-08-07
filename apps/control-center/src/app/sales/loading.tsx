export default function SalesLoading() {
  return <div className="page-stack" aria-label="Loading live sales"><div className="skeleton skeleton-heading" /><div className="metric-grid">{Array.from({ length: 4 }, (_, index) => <div className="skeleton skeleton-card" key={index} />)}</div><div className="skeleton skeleton-panel" /></div>;
}
