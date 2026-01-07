/**
 * Welcome Page
 * Wallet creation and import flow
 */

import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useWalletStore } from '@/stores/wallet';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Modal } from '@/components/ui/Modal';
import toast from 'react-hot-toast';

type Step = 'welcome' | 'create' | 'import' | 'backup';

const Welcome: React.FC = () => {
  const navigate = useNavigate();
  const { createWallet, importWallet, isLoading } = useWalletStore();

  const [step, setStep] = useState<Step>('welcome');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [mnemonic, setMnemonic] = useState('');
  const [generatedMnemonic, setGeneratedMnemonic] = useState('');
  const [hasBackedUp, setHasBackedUp] = useState(false);
  const [showMnemonic, setShowMnemonic] = useState(false);

  const validatePassword = (): boolean => {
    if (password.length < 8) {
      toast.error('Password must be at least 8 characters');
      return false;
    }
    if (password !== confirmPassword) {
      toast.error('Passwords do not match');
      return false;
    }
    return true;
  };

  const handleCreate = async () => {
    if (!validatePassword()) return;

    try {
      const newMnemonic = await createWallet(password);
      setGeneratedMnemonic(newMnemonic);
      setStep('backup');
    } catch (error) {
      console.error('Create wallet error:', error);
      const message = error instanceof Error ? error.message : 'Unknown error';
      toast.error(`Failed to create wallet: ${message}`);
    }
  };

  const handleImport = async () => {
    if (!validatePassword()) return;

    const words = mnemonic.trim().split(/\s+/);
    if (words.length !== 12 && words.length !== 24) {
      toast.error('Invalid recovery phrase. Must be 12 or 24 words.');
      return;
    }

    try {
      await importWallet(mnemonic.trim(), password);
      toast.success('Wallet imported successfully');
      navigate('/');
    } catch (error) {
      console.error('Import wallet error:', error);
      const message = error instanceof Error ? error.message : 'Unknown error';
      toast.error(`Failed to import wallet: ${message}`);
    }
  };

  const handleBackupComplete = () => {
    if (!hasBackedUp) {
      toast.error('Please confirm you have saved your recovery phrase');
      return;
    }
    toast.success('Wallet created successfully');
    navigate('/');
  };

  const copyMnemonic = () => {
    navigator.clipboard.writeText(generatedMnemonic);
    toast.success('Recovery phrase copied to clipboard');
  };

  // Welcome screen
  if (step === 'welcome') {
    return (
      <div className="min-h-screen bg-dark-950 flex items-center justify-center p-4">
        <div className="w-full max-w-md text-center">
          {/* Logo */}
          <div className="mb-8">
            <div className="inline-flex items-center justify-center w-24 h-24 rounded-3xl bg-gradient-to-br from-omniphi-500 to-omniphi-700 mb-6 shadow-lg shadow-omniphi-500/20">
              <span className="text-5xl font-bold text-white">O</span>
            </div>
            <h1 className="text-3xl font-bold text-dark-100 mb-2">Omniphi Wallet</h1>
            <p className="text-dark-400">Your gateway to the Omniphi ecosystem</p>
          </div>

          {/* Action buttons */}
          <div className="space-y-4">
            <Button
              variant="primary"
              size="lg"
              className="w-full"
              onClick={() => setStep('create')}
            >
              Create New Wallet
            </Button>
            <Button
              variant="secondary"
              size="lg"
              className="w-full"
              onClick={() => setStep('import')}
            >
              Import Existing Wallet
            </Button>
          </div>

          {/* Info */}
          <div className="mt-12 grid grid-cols-3 gap-4 text-center">
            <div className="p-4">
              <div className="w-10 h-10 rounded-lg bg-omniphi-600/20 flex items-center justify-center mx-auto mb-2">
                <svg className="w-5 h-5 text-omniphi-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                </svg>
              </div>
              <p className="text-xs text-dark-400">Secure</p>
            </div>
            <div className="p-4">
              <div className="w-10 h-10 rounded-lg bg-omniphi-600/20 flex items-center justify-center mx-auto mb-2">
                <svg className="w-5 h-5 text-omniphi-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
                </svg>
              </div>
              <p className="text-xs text-dark-400">Fast</p>
            </div>
            <div className="p-4">
              <div className="w-10 h-10 rounded-lg bg-omniphi-600/20 flex items-center justify-center mx-auto mb-2">
                <svg className="w-5 h-5 text-omniphi-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
                </svg>
              </div>
              <p className="text-xs text-dark-400">Trusted</p>
            </div>
          </div>
        </div>
      </div>
    );
  }

  // Create wallet screen
  if (step === 'create') {
    return (
      <div className="min-h-screen bg-dark-950 flex items-center justify-center p-4">
        <div className="w-full max-w-md">
          <button
            onClick={() => setStep('welcome')}
            className="flex items-center gap-2 text-dark-400 hover:text-dark-200 mb-6"
          >
            <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
            </svg>
            Back
          </button>

          <h1 className="text-2xl font-bold text-dark-100 mb-2">Create New Wallet</h1>
          <p className="text-dark-400 mb-8">Set a strong password to protect your wallet</p>

          <form onSubmit={(e) => { e.preventDefault(); handleCreate(); }} className="space-y-4">
            <div>
              <label className="label">Password</label>
              <Input
                type="password"
                placeholder="Enter password (min 8 characters)"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </div>
            <div>
              <label className="label">Confirm Password</label>
              <Input
                type="password"
                placeholder="Confirm your password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
              />
            </div>

            <div className="pt-4">
              <Button
                type="submit"
                variant="primary"
                size="lg"
                className="w-full"
                isLoading={isLoading}
              >
                Create Wallet
              </Button>
            </div>
          </form>
        </div>
      </div>
    );
  }

  // Import wallet screen
  if (step === 'import') {
    return (
      <div className="min-h-screen bg-dark-950 flex items-center justify-center p-4">
        <div className="w-full max-w-md">
          <button
            onClick={() => setStep('welcome')}
            className="flex items-center gap-2 text-dark-400 hover:text-dark-200 mb-6"
          >
            <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
            </svg>
            Back
          </button>

          <h1 className="text-2xl font-bold text-dark-100 mb-2">Import Wallet</h1>
          <p className="text-dark-400 mb-8">Enter your 12 or 24 word recovery phrase</p>

          <form onSubmit={(e) => { e.preventDefault(); handleImport(); }} className="space-y-4">
            <div>
              <label className="label">Recovery Phrase</label>
              <textarea
                className="input min-h-[120px] resize-none"
                placeholder="Enter your recovery phrase..."
                value={mnemonic}
                onChange={(e) => setMnemonic(e.target.value)}
              />
              <p className="text-xs text-dark-500 mt-1">
                Words should be separated by spaces
              </p>
            </div>
            <div>
              <label className="label">Password</label>
              <Input
                type="password"
                placeholder="Enter password (min 8 characters)"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </div>
            <div>
              <label className="label">Confirm Password</label>
              <Input
                type="password"
                placeholder="Confirm your password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
              />
            </div>

            <div className="pt-4">
              <Button
                type="submit"
                variant="primary"
                size="lg"
                className="w-full"
                isLoading={isLoading}
              >
                Import Wallet
              </Button>
            </div>
          </form>
        </div>
      </div>
    );
  }

  // Backup screen
  if (step === 'backup') {
    return (
      <div className="min-h-screen bg-dark-950 flex items-center justify-center p-4">
        <div className="w-full max-w-lg">
          <div className="text-center mb-8">
            <div className="inline-flex items-center justify-center w-16 h-16 rounded-full bg-yellow-500/10 text-yellow-400 mb-4">
              <svg className="w-8 h-8" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
            </div>
            <h1 className="text-2xl font-bold text-dark-100 mb-2">Backup Your Wallet</h1>
            <p className="text-dark-400">
              Write down these words in order and store them safely.
              <br />
              <span className="text-red-400">Never share your recovery phrase with anyone.</span>
            </p>
          </div>

          {/* Mnemonic display */}
          <div className="card mb-6">
            <div className="flex items-center justify-between mb-4">
              <span className="text-sm text-dark-400">Recovery Phrase</span>
              <div className="flex items-center gap-2">
                <button
                  onClick={() => setShowMnemonic(!showMnemonic)}
                  className="text-sm text-omniphi-400 hover:text-omniphi-300"
                >
                  {showMnemonic ? 'Hide' : 'Show'}
                </button>
                <button
                  onClick={copyMnemonic}
                  className="text-sm text-omniphi-400 hover:text-omniphi-300"
                >
                  Copy
                </button>
              </div>
            </div>

            <div className={`grid grid-cols-3 gap-2 ${!showMnemonic ? 'blur-md select-none' : ''}`}>
              {generatedMnemonic.split(' ').map((word, index) => (
                <div
                  key={index}
                  className="flex items-center gap-2 px-3 py-2 bg-dark-800 rounded-lg"
                >
                  <span className="text-xs text-dark-500 w-5">{index + 1}.</span>
                  <span className="font-mono text-dark-100">{word}</span>
                </div>
              ))}
            </div>
          </div>

          {/* Warning */}
          <div className="bg-red-500/10 border border-red-500/20 rounded-lg p-4 mb-6">
            <div className="flex gap-3">
              <svg className="w-5 h-5 text-red-400 flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
              <div className="text-sm text-red-300">
                <strong>Warning:</strong> If you lose your recovery phrase, you will lose access to your wallet and funds. We cannot recover it for you.
              </div>
            </div>
          </div>

          {/* Confirmation */}
          <label className="flex items-center gap-3 mb-6 cursor-pointer">
            <input
              type="checkbox"
              checked={hasBackedUp}
              onChange={(e) => setHasBackedUp(e.target.checked)}
              className="w-5 h-5 rounded border-dark-600 bg-dark-800 text-omniphi-600 focus:ring-omniphi-500 focus:ring-offset-dark-900"
            />
            <span className="text-dark-300">
              I have safely stored my recovery phrase
            </span>
          </label>

          <Button
            variant="primary"
            size="lg"
            className="w-full"
            onClick={handleBackupComplete}
            disabled={!hasBackedUp}
          >
            Continue to Wallet
          </Button>
        </div>
      </div>
    );
  }

  return null;
};

export default Welcome;
