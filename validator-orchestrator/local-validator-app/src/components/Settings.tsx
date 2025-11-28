import { useState } from 'react';
import { ValidatorConfig } from '../types';

interface Props {
  config: ValidatorConfig;
  onUpdate: (config: Partial<ValidatorConfig>) => Promise<void>;
}

export default function Settings({ config, onUpdate }: Props) {
  const [formData, setFormData] = useState(config);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState('');

  const handleSave = async () => {
    setSaving(true);
    setMessage('');

    try {
      await onUpdate(formData);
      setMessage('Settings saved successfully');
    } catch (error) {
      setMessage(`Error: ${error}`);
    } finally {
      setSaving(false);
    }
  };

  const handleReset = () => {
    setFormData(config);
    setMessage('');
  };

  return (
    <div className="card settings">
      <h2>Settings</h2>

      <div className="settings-form">
        <div className="form-group">
          <label htmlFor="moniker">Validator Moniker</label>
          <input
            id="moniker"
            type="text"
            value={formData.moniker}
            onChange={(e) => setFormData({ ...formData, moniker: e.target.value })}
            className="input-field"
          />
          <small>Display name for your validator</small>
        </div>

        <div className="form-group">
          <label htmlFor="chainId">Chain ID</label>
          <input
            id="chainId"
            type="text"
            value={formData.chainId}
            onChange={(e) => setFormData({ ...formData, chainId: e.target.value })}
            className="input-field"
          />
          <small>The blockchain network to connect to</small>
        </div>

        <div className="form-group">
          <label htmlFor="orchestratorUrl">Orchestrator URL</label>
          <input
            id="orchestratorUrl"
            type="text"
            value={formData.orchestratorUrl}
            onChange={(e) => setFormData({ ...formData, orchestratorUrl: e.target.value })}
            className="input-field"
          />
          <small>Backend orchestrator API endpoint</small>
        </div>

        <div className="form-group">
          <label htmlFor="heartbeatInterval">Heartbeat Interval (seconds)</label>
          <input
            id="heartbeatInterval"
            type="number"
            value={formData.heartbeatInterval / 1000}
            onChange={(e) => setFormData({ ...formData, heartbeatInterval: parseInt(e.target.value) * 1000 })}
            className="input-field"
            min="10"
            max="300"
          />
          <small>How often to send status updates to orchestrator</small>
        </div>

        <div className="form-group">
          <label>
            <input
              type="checkbox"
              checked={formData.autoStart}
              onChange={(e) => setFormData({ ...formData, autoStart: e.target.checked })}
            />
            Auto-start validator on app launch
          </label>
        </div>

        <div className="form-actions">
          <button onClick={handleSave} className="btn-primary" disabled={saving}>
            {saving ? 'Saving...' : 'Save Settings'}
          </button>
          <button onClick={handleReset} className="btn-secondary">
            Reset
          </button>
        </div>

        {message && (
          <div className={`message ${message.includes('Error') ? 'error' : 'success'}`}>
            {message}
          </div>
        )}
      </div>
    </div>
  );
}
