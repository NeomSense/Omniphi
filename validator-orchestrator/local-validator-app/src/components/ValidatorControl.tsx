import { useState } from 'react';
import { ValidatorStatus, ValidatorConfig } from '../types';

interface Props {
  status: ValidatorStatus;
  config: ValidatorConfig;
}

export default function ValidatorControl({ status, config }: Props) {
  const [starting, setStarting] = useState(false);
  const [stopping, setStopping] = useState(false);
  const [message, setMessage] = useState('');

  const handleStart = async () => {
    setStarting(true);
    setMessage('');

    try {
      const result = await window.electronAPI.startValidator({
        chainId: config.chainId,
        moniker: config.moniker
      });

      if (result.success) {
        setMessage('Validator started successfully');
      } else {
        setMessage(`Error: ${result.error}`);
      }
    } catch (error) {
      setMessage(`Error: ${error}`);
    } finally {
      setStarting(false);
    }
  };

  const handleStop = async () => {
    setStopping(true);
    setMessage('');

    try {
      const result = await window.electronAPI.stopValidator();

      if (result.success) {
        setMessage('Validator stopped');
      } else {
        setMessage(`Error: ${result.error}`);
      }
    } catch (error) {
      setMessage(`Error: ${error}`);
    } finally {
      setStopping(false);
    }
  };

  return (
    <div className="card validator-control">
      <h2>Validator Control</h2>

      <div className="control-buttons">
        {!status.running ? (
          <button
            className="btn-primary btn-large"
            onClick={handleStart}
            disabled={starting}
          >
            {starting ? 'Starting...' : 'Start Validator'}
          </button>
        ) : (
          <button
            className="btn-danger btn-large"
            onClick={handleStop}
            disabled={stopping}
          >
            {stopping ? 'Stopping...' : 'Stop Validator'}
          </button>
        )}
      </div>

      {message && (
        <div className={`message ${message.includes('Error') ? 'error' : 'success'}`}>
          {message}
        </div>
      )}

      <div className="info-grid">
        <div className="info-item">
          <span className="label">Moniker:</span>
          <span className="value">{config.moniker}</span>
        </div>
        <div className="info-item">
          <span className="label">Chain ID:</span>
          <span className="value">{config.chainId}</span>
        </div>
      </div>
    </div>
  );
}
