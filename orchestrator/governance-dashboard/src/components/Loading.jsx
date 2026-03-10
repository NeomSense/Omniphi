export default function Loading({ message = 'Loading…' }) {
  return (
    <div className="loading">
      <div className="spinner" />
      <span>{message}</span>
    </div>
  );
}
