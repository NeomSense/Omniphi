import { useState } from 'react';

export default function JsonViewer({ label, data }) {
  const [open, setOpen] = useState(false);

  if (!data) return null;

  return (
    <div className="json-viewer">
      <button
        className="json-toggle"
        onClick={() => setOpen((v) => !v)}
      >
        {open ? '▾' : '▸'} {label || 'Raw JSON'}
      </button>
      {open && (
        <pre className="json-pre">
          {JSON.stringify(data, null, 2)}
        </pre>
      )}
    </div>
  );
}
