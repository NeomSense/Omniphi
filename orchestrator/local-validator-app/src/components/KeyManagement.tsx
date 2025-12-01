import { useState, useEffect } from 'react';

export default function KeyManagement() {
  const [pubkey, setPubkey] = useState('');
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState('');
  const [exportPassword, setExportPassword] = useState('');
  const [importing, setImporting] = useState(false);

  useEffect(() => {
    loadPubkey();
  }, []);

  const loadPubkey = async () => {
    setLoading(true);
    const result = await window.electronAPI.getConsensusPubkey();

    if (result.success && result.pubkey) {
      setPubkey(JSON.stringify(result.pubkey, null, 2));
    } else {
      setPubkey('No consensus key found');
    }
    setLoading(false);
  };

  const exportKey = async () => {
    if (!exportPassword) {
      setMessage('Please enter a password to encrypt the key');
      return;
    }

    setLoading(true);
    setMessage('');

    const result = await window.electronAPI.exportPrivateKey(exportPassword);

    if (result.success && result.encryptedKey) {
      // Download as file
      const blob = new Blob([result.encryptedKey], { type: 'text/plain' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'omniphi-validator-key-backup.txt';
      a.click();
      URL.revokeObjectURL(url);

      setMessage('Key exported successfully');
    } else {
      setMessage(`Error: ${result.error}`);
    }

    setLoading(false);
  };

  const copyPubkey = () => {
    navigator.clipboard.writeText(pubkey);
    setMessage('Public key copied to clipboard');
    setTimeout(() => setMessage(''), 3000);
  };

  return (
    <div className="card key-management">
      <h2>Consensus Key Management</h2>

      <div className="key-section">
        <h3>Consensus Public Key</h3>
        <p>Use this public key when creating your validator on-chain.</p>

        <div className="pubkey-display">
          <pre>{loading ? 'Loading...' : pubkey}</pre>
        </div>

        <div className="key-actions">
          <button onClick={loadPubkey} className="btn-secondary" disabled={loading}>
            Refresh
          </button>
          <button onClick={copyPubkey} className="btn-secondary" disabled={!pubkey || pubkey === 'No consensus key found'}>
            Copy to Clipboard
          </button>
        </div>
      </div>

      <div className="key-section">
        <h3>Backup Private Key</h3>
        <p className="warning">Warning: Keep your backup secure! Never share your private key.</p>

        <div className="export-form">
          <input
            type="password"
            placeholder="Enter encryption password"
            value={exportPassword}
            onChange={(e) => setExportPassword(e.target.value)}
            className="input-field"
          />
          <button onClick={exportKey} className="btn-primary" disabled={loading}>
            Export Encrypted Backup
          </button>
        </div>
      </div>

      {message && (
        <div className={`message ${message.includes('Error') ? 'error' : 'success'}`}>
          {message}
        </div>
      )}
    </div>
  );
}
