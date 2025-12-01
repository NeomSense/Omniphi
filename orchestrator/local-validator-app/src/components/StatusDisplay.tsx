import { useState, useEffect } from 'react';
import { ValidatorStatus, ValidatorConfig } from '../types';

interface Props {
  status: ValidatorStatus;
  config: ValidatorConfig;
}

export default function StatusDisplay({ status, config }: Props) {
  const [walletAddress, setWalletAddress] = useState('');
  const [sending, setSending] = useState(false);
  const [heartbeatMessage, setHeartbeatMessage] = useState('');

  const formatUptime = (seconds: number): string => {
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const secs = seconds % 60;
    return `${hours}h ${minutes}m ${secs}s`;
  };

  const sendHeartbeat = async () => {
    if (!walletAddress.trim()) {
      setHeartbeatMessage('Please enter wallet address');
      return;
    }

    setSending(true);
    setHeartbeatMessage('');

    try {
      const result = await window.electronAPI.sendHeartbeat({
        walletAddress: walletAddress.trim()
      });

      if (result.success) {
        setHeartbeatMessage('Heartbeat sent successfully');
      } else {
        setHeartbeatMessage(`Error: ${result.error}`);
      }
    } catch (error) {
      setHeartbeatMessage(`Error: ${error}`);
    } finally {
      setSending(false);
    }
  };

  return (
    <div className="card status-display">
      <h2>Validator Status</h2>

      <div className="status-grid">
        <div className="status-item">
          <div className="status-label">Block Height</div>
          <div className="status-value large">{status.blockHeight.toLocaleString()}</div>
        </div>

        <div className="status-item">
          <div className="status-label">Peers</div>
          <div className="status-value large">{status.peers}</div>
        </div>

        <div className="status-item">
          <div className="status-label">Syncing</div>
          <div className="status-value">
            <span className={status.syncing ? 'badge warning' : 'badge success'}>
              {status.syncing ? 'Yes' : 'Synced'}
            </span>
          </div>
        </div>

        <div className="status-item">
          <div className="status-label">Uptime</div>
          <div className="status-value">{formatUptime(status.uptime)}</div>
        </div>
      </div>

      {status.running && (
        <div className="heartbeat-section">
          <h3>Send Heartbeat to Orchestrator</h3>
          <p>Send your validator status to the orchestrator backend at: {config.orchestratorUrl}</p>

          <div className="heartbeat-form">
            <input
              type="text"
              placeholder="Enter your wallet address (omni...)"
              value={walletAddress}
              onChange={(e) => setWalletAddress(e.target.value)}
              className="input-field"
            />
            <button
              onClick={sendHeartbeat}
              disabled={sending || !status.running}
              className="btn-primary"
            >
              {sending ? 'Sending...' : 'Send Heartbeat'}
            </button>
          </div>

          {heartbeatMessage && (
            <div className={`message ${heartbeatMessage.includes('Error') ? 'error' : 'success'}`}>
              {heartbeatMessage}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
