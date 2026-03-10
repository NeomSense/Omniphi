export default function ErrorBanner({ error, onRetry }) {
  if (!error) return null;

  const msg = error?.message || String(error);

  return (
    <div className="error-banner">
      <span className="error-icon">⚠</span>
      <span className="error-msg">{msg}</span>
      {onRetry && (
        <button className="btn btn-sm" onClick={onRetry}>
          Retry
        </button>
      )}
    </div>
  );
}
