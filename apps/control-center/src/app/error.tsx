"use client";
export default function ErrorPage({ reset }: { error: Error & { digest?: string }; reset: () => void }) { return <div className="error-state"><span>!</span><h1>Something went wrong / Hitilafu imetokea</h1><p>We could not load this workspace. Your data has not been changed.</p><button className="primary-button" type="button" onClick={reset}>Try again / Jaribu tena</button></div>; }
