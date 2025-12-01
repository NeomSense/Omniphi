import { useState } from 'react';
import { Card } from '../ui/Card';
import { Badge } from '../ui/Badge';
import { ConsensusKey } from '../../types/validator';
import { clsx } from 'clsx';

interface KeysPageProps {
  onBack?: () => void;
}

interface KeyInfo {
  name: string;
  type: 'validator' | 'operator' | 'node';
  address: string;
  pubkey: string;
  created: string;
  lastUsed: string;
  backed_up: boolean;
}

export function KeysPage({ onBack }: KeysPageProps) {
  const [keys, setKeys] = useState<KeyInfo[]>([
    {
      name: 'validator-key',
      type: 'validator',
      address: 'omnivalcons1z2rnzs9s5ga8v0nceuky2hqphx8gxms3abc123',
      pubkey: 'omnivalconspub1zcjduepqw4uxlzr8lm9pe7zy3c6f9xqy5z...',
      created: '2024-01-15T10:30:00Z',
      lastUsed: new Date().toISOString(),
      backed_up: true,
    },
    {
      name: 'operator-key',
      type: 'operator',
      address: 'omni1z2rnzs9s5ga8v0nceuky2hqphx8gxms3w6a9qp',
      pubkey: 'omnipub1addwnpepqw4uxlzr8lm9pe7zy3c6f9xqy5z...',
      created: '2024-01-15T10:30:00Z',
      lastUsed: new Date().toISOString(),
      backed_up: true,
    },
    {
      name: 'node-key',
      type: 'node',
      address: 'f632a7ee6f28d12cde86d009ba0cc614795bf59f',
      pubkey: 'ed25519:abc123def456...',
      created: '2024-01-15T10:30:00Z',
      lastUsed: new Date().toISOString(),
      backed_up: false,
    },
  ]);

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showBackupModal, setShowBackupModal] = useState(false);
  const [selectedKey, setSelectedKey] = useState<KeyInfo | null>(null);
  const [showPrivateKey, setShowPrivateKey] = useState(false);

  const getTypeColor = (type: string) => {
    switch (type) {
      case 'validator': return 'bg-purple-100 text-purple-800';
      case 'operator': return 'bg-blue-100 text-blue-800';
      case 'node': return 'bg-green-100 text-green-800';
      default: return 'bg-gray-100 text-gray-800';
    }
  };

  const truncateKey = (key: string, start = 20, end = 8) => {
    if (key.length <= start + end) return key;
    return `${key.slice(0, start)}...${key.slice(-end)}`;
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
  };

  const handleBackup = (key: KeyInfo) => {
    setSelectedKey(key);
    setShowBackupModal(true);
  };

  const handleExportKey = () => {
    if (!selectedKey) return;

    // In production, this would trigger secure key export via IPC
    const exportData = {
      name: selectedKey.name,
      type: selectedKey.type,
      address: selectedKey.address,
      pubkey: selectedKey.pubkey,
      exported_at: new Date().toISOString(),
    };

    const blob = new Blob([JSON.stringify(exportData, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${selectedKey.name}-backup.json`;
    a.click();
    URL.revokeObjectURL(url);

    // Mark as backed up
    setKeys(prev => prev.map(k =>
      k.name === selectedKey.name ? { ...k, backed_up: true } : k
    ));
    setShowBackupModal(false);
  };

  return (
    <div className="min-h-screen bg-gray-50 flex flex-col">
      {/* Header */}
      <header className="bg-white shadow-sm border-b border-gray-200">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-4">
              {onBack && (
                <button onClick={onBack} className="text-gray-500 hover:text-gray-700">
                  <span className="text-xl">&larr;</span>
                </button>
              )}
              <div>
                <h1 className="text-xl font-bold text-gray-900">Key Management</h1>
                <p className="text-sm text-gray-500">Secure management of validator keys</p>
              </div>
            </div>
            <button
              onClick={() => setShowCreateModal(true)}
              className="btn btn-primary"
            >
              + Create New Key
            </button>
          </div>
        </div>
      </header>

      {/* Security Warning */}
      <div className="bg-yellow-50 border-b border-yellow-200 px-4 sm:px-6 lg:px-8 py-3">
        <div className="max-w-7xl mx-auto flex items-center space-x-3">
          <span className="text-yellow-600 text-xl">⚠️</span>
          <p className="text-sm text-yellow-800">
            <strong>Security Notice:</strong> Never share your private keys. Always keep secure backups in multiple locations.
          </p>
        </div>
      </div>

      {/* Main Content */}
      <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div className="grid gap-6">
          {keys.map((key, idx) => (
            <Card key={idx}>
              <div className="flex items-start justify-between">
                <div className="flex-1">
                  <div className="flex items-center space-x-3 mb-3">
                    <h3 className="text-lg font-semibold text-gray-900">{key.name}</h3>
                    <span className={clsx('px-2 py-1 rounded text-xs font-medium', getTypeColor(key.type))}>
                      {key.type.toUpperCase()}
                    </span>
                    {key.backed_up ? (
                      <Badge variant="success">Backed Up</Badge>
                    ) : (
                      <Badge variant="warning">Not Backed Up</Badge>
                    )}
                  </div>

                  {/* Address */}
                  <div className="mb-3">
                    <p className="text-xs text-gray-500 mb-1">Address</p>
                    <div className="flex items-center space-x-2">
                      <code className="text-sm bg-gray-100 px-3 py-1.5 rounded font-mono">
                        {truncateKey(key.address)}
                      </code>
                      <button
                        onClick={() => copyToClipboard(key.address)}
                        className="text-purple-600 hover:text-purple-700 text-sm"
                      >
                        Copy
                      </button>
                    </div>
                  </div>

                  {/* Public Key */}
                  <div className="mb-3">
                    <p className="text-xs text-gray-500 mb-1">Public Key</p>
                    <div className="flex items-center space-x-2">
                      <code className="text-sm bg-gray-100 px-3 py-1.5 rounded font-mono">
                        {truncateKey(key.pubkey)}
                      </code>
                      <button
                        onClick={() => copyToClipboard(key.pubkey)}
                        className="text-purple-600 hover:text-purple-700 text-sm"
                      >
                        Copy
                      </button>
                    </div>
                  </div>

                  {/* Metadata */}
                  <div className="flex items-center space-x-6 text-xs text-gray-500">
                    <span>Created: {new Date(key.created).toLocaleDateString()}</span>
                    <span>Last Used: {new Date(key.lastUsed).toLocaleString()}</span>
                  </div>
                </div>

                {/* Actions */}
                <div className="flex flex-col space-y-2">
                  <button
                    onClick={() => handleBackup(key)}
                    className="btn btn-secondary text-sm"
                  >
                    Backup
                  </button>
                  <button className="btn btn-secondary text-sm">
                    Rotate
                  </button>
                  {key.type !== 'validator' && (
                    <button className="text-red-600 hover:text-red-700 text-sm font-medium">
                      Delete
                    </button>
                  )}
                </div>
              </div>
            </Card>
          ))}
        </div>

        {/* Key Security Best Practices */}
        <div className="mt-8">
          <Card title="Key Security Best Practices">
            <div className="grid md:grid-cols-2 gap-4">
              <div className="p-4 bg-green-50 rounded-lg">
                <h4 className="font-medium text-green-800 mb-2">Do</h4>
                <ul className="text-sm text-green-700 space-y-1">
                  <li>- Keep multiple encrypted backups</li>
                  <li>- Store backups in separate physical locations</li>
                  <li>- Use hardware security modules (HSM) for mainnet</li>
                  <li>- Regularly verify backup integrity</li>
                  <li>- Use strong passphrases</li>
                </ul>
              </div>
              <div className="p-4 bg-red-50 rounded-lg">
                <h4 className="font-medium text-red-800 mb-2">Don't</h4>
                <ul className="text-sm text-red-700 space-y-1">
                  <li>- Never share private keys</li>
                  <li>- Don't store keys in plain text</li>
                  <li>- Don't email or message key files</li>
                  <li>- Don't use the same key for multiple validators</li>
                  <li>- Don't ignore backup warnings</li>
                </ul>
              </div>
            </div>
          </Card>
        </div>
      </main>

      {/* Backup Modal */}
      {showBackupModal && selectedKey && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white rounded-xl shadow-xl max-w-md w-full mx-4 p-6">
            <h3 className="text-lg font-bold text-gray-900 mb-4">Backup Key: {selectedKey.name}</h3>

            <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-4 mb-4">
              <p className="text-sm text-yellow-800">
                This will export your key data. Store it securely and never share it with anyone.
              </p>
            </div>

            <div className="space-y-3 mb-6">
              <div>
                <p className="text-xs text-gray-500 mb-1">Key Name</p>
                <p className="font-medium">{selectedKey.name}</p>
              </div>
              <div>
                <p className="text-xs text-gray-500 mb-1">Type</p>
                <p className="font-medium capitalize">{selectedKey.type}</p>
              </div>
              <div>
                <p className="text-xs text-gray-500 mb-1">Address</p>
                <code className="text-sm bg-gray-100 px-2 py-1 rounded">{truncateKey(selectedKey.address)}</code>
              </div>
            </div>

            <div className="flex space-x-3">
              <button
                onClick={() => setShowBackupModal(false)}
                className="flex-1 btn btn-secondary"
              >
                Cancel
              </button>
              <button
                onClick={handleExportKey}
                className="flex-1 btn btn-primary"
              >
                Export Backup
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
