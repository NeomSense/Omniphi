import { useState, useEffect } from 'react';

export default function LogViewer() {
  const [logs, setLogs] = useState<string[]>([]);
  const [autoScroll, setAutoScroll] = useState(true);

  useEffect(() => {
    loadLogs();

    // Listen for new logs
    window.electronAPI.onLogUpdate((log: string) => {
      setLogs(prev => [...prev.slice(-999), log]); // Keep last 1000 lines
    });

    return () => {
      window.electronAPI.removeLogListener();
    };
  }, []);

  const loadLogs = async () => {
    const result = await window.electronAPI.getValidatorLogs(100);
    setLogs(result.map(entry => `[${entry.timestamp}] ${entry.message}`));
  };

  const clearLogs = () => {
    setLogs([]);
  };

  return (
    <div className="card log-viewer">
      <div className="log-header">
        <h2>Validator Logs</h2>
        <div className="log-controls">
          <label>
            <input
              type="checkbox"
              checked={autoScroll}
              onChange={(e) => setAutoScroll(e.target.checked)}
            />
            Auto-scroll
          </label>
          <button onClick={loadLogs} className="btn-secondary">Refresh</button>
          <button onClick={clearLogs} className="btn-secondary">Clear</button>
        </div>
      </div>

      <div className="log-content">
        {logs.length === 0 ? (
          <div className="log-empty">No logs available. Start the validator to see logs.</div>
        ) : (
          logs.map((log, index) => (
            <div key={index} className="log-line">{log}</div>
          ))
        )}
      </div>
    </div>
  );
}
