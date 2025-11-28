/**
 * UpgradeNotification Component
 *
 * Displays a banner when a new version is available.
 * Checks for updates periodically and provides upgrade actions.
 */

import { useState, useEffect, useCallback } from 'react';
import { updatesApi } from '../../services/api';

interface UpdateInfo {
  available: boolean;
  version?: string;
  releaseNotes?: string;
}

interface UpgradeNotificationProps {
  /** Check interval in milliseconds (default: 1 hour) */
  checkInterval?: number;
  /** Whether to show the notification (can be dismissed) */
  show?: boolean;
  /** Callback when notification is dismissed */
  onDismiss?: () => void;
}

export function UpgradeNotification({
  checkInterval = 3600000, // 1 hour
  show = true,
  onDismiss,
}: UpgradeNotificationProps) {
  const [updateInfo, setUpdateInfo] = useState<UpdateInfo | null>(null);
  const [dismissed, setDismissed] = useState(false);
  const [showNotes, setShowNotes] = useState(false);
  const [checking, setChecking] = useState(false);

  // Check for updates
  const checkForUpdates = useCallback(async () => {
    setChecking(true);
    try {
      const result = await updatesApi.checkForUpdates();
      if (result.success && result.data) {
        setUpdateInfo(result.data);
        // Reset dismissed if a new version is found
        if (result.data.version !== updateInfo?.version) {
          setDismissed(false);
        }
      }
    } catch (error) {
      console.error('Failed to check for updates:', error);
    } finally {
      setChecking(false);
    }
  }, [updateInfo?.version]);

  // Initial check and periodic checks
  useEffect(() => {
    checkForUpdates();

    const interval = setInterval(checkForUpdates, checkInterval);
    return () => clearInterval(interval);
  }, [checkForUpdates, checkInterval]);

  // Handle dismiss
  const handleDismiss = () => {
    setDismissed(true);
    onDismiss?.();
  };

  // Don't render if no update or dismissed
  if (!show || dismissed || !updateInfo?.available) {
    return null;
  }

  return (
    <div className="bg-gradient-to-r from-purple-600 to-indigo-600 text-white">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="py-3">
          <div className="flex items-center justify-between flex-wrap">
            <div className="flex-1 flex items-center">
              <span className="flex p-2 rounded-lg bg-purple-800">
                <svg
                  className="h-5 w-5 text-white"
                  xmlns="http://www.w3.org/2000/svg"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12"
                  />
                </svg>
              </span>
              <p className="ml-3 font-medium truncate">
                <span className="md:hidden">New version available!</span>
                <span className="hidden md:inline">
                  A new version of Omniphi Validator is available: v{updateInfo.version}
                </span>
              </p>
            </div>

            <div className="flex items-center space-x-4 mt-2 sm:mt-0 sm:ml-4">
              {updateInfo.releaseNotes && (
                <button
                  onClick={() => setShowNotes(!showNotes)}
                  className="flex items-center px-3 py-1 text-sm font-medium text-purple-200 hover:text-white transition-colors"
                >
                  {showNotes ? 'Hide' : 'View'} Release Notes
                </button>
              )}

              <button
                onClick={() => {
                  // In production, this would trigger the update process
                  alert('Update process would start here');
                }}
                className="flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-purple-600 bg-white hover:bg-purple-50 transition-colors"
              >
                Update Now
              </button>

              <button
                onClick={handleDismiss}
                className="flex-shrink-0 rounded-md p-1 hover:bg-purple-500 focus:outline-none focus:ring-2 focus:ring-white transition-colors"
                aria-label="Dismiss"
              >
                <svg
                  className="h-5 w-5"
                  xmlns="http://www.w3.org/2000/svg"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M6 18L18 6M6 6l12 12"
                  />
                </svg>
              </button>
            </div>
          </div>

          {/* Release Notes Expandable Section */}
          {showNotes && updateInfo.releaseNotes && (
            <div className="mt-3 pt-3 border-t border-purple-500">
              <h4 className="text-sm font-semibold text-purple-200 mb-2">
                Release Notes for v{updateInfo.version}
              </h4>
              <div className="bg-purple-800 rounded-lg p-4 text-sm text-purple-100 max-h-48 overflow-y-auto">
                <pre className="whitespace-pre-wrap font-sans">
                  {updateInfo.releaseNotes}
                </pre>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

/**
 * Compact version of the upgrade notification for sidebar/footer use
 */
export function UpgradeNotificationCompact() {
  const [updateInfo, setUpdateInfo] = useState<UpdateInfo | null>(null);

  useEffect(() => {
    const checkUpdate = async () => {
      const result = await updatesApi.checkForUpdates();
      if (result.success && result.data) {
        setUpdateInfo(result.data);
      }
    };
    checkUpdate();
  }, []);

  if (!updateInfo?.available) {
    return null;
  }

  return (
    <div className="p-3 bg-purple-50 border border-purple-200 rounded-lg">
      <div className="flex items-center space-x-2">
        <div className="flex-shrink-0">
          <span className="inline-flex items-center justify-center h-8 w-8 rounded-full bg-purple-100">
            <svg
              className="h-4 w-4 text-purple-600"
              xmlns="http://www.w3.org/2000/svg"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12"
              />
            </svg>
          </span>
        </div>
        <div className="flex-1 min-w-0">
          <p className="text-sm font-medium text-purple-900">
            Update Available
          </p>
          <p className="text-xs text-purple-600">
            v{updateInfo.version}
          </p>
        </div>
        <button
          onClick={() => alert('Update process would start here')}
          className="px-2 py-1 text-xs font-medium text-purple-700 bg-purple-100 rounded hover:bg-purple-200 transition-colors"
        >
          Update
        </button>
      </div>
    </div>
  );
}

export default UpgradeNotification;
