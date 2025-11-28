import { useState, useEffect, useRef } from 'react';
import { LogEntry } from '../../types/validator';
import { clsx } from 'clsx';

interface LogsPageProps {
  onBack?: () => void;
}

export function LogsPage({ onBack }: LogsPageProps) {
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [filter, setFilter] = useState<'all' | 'info' | 'warn' | 'error' | 'debug'>('all');
  const [search, setSearch] = useState('');
  const [autoScroll, setAutoScroll] = useState(true);
  const [isPaused, setIsPaused] = useState(false);
  const logContainerRef = useRef<HTMLDivElement>(null);

  // Generate mock logs for demo
  useEffect(() => {
    const sources: LogEntry['source'][] = ['tendermint', 'abci', 'app', 'upgrade'];
    const levels: LogEntry['level'][] = ['info', 'debug', 'warn', 'error'];
    const messages = [
      'Committed state',
      'Executed block',
      'Received proposal',
      'Applied snapshot chunk',
      'Indexed block events',
      'Peer connected',
      'Peer disconnected',
      'Consensus timeout',
      'Received vote',
      'Finalized commit',
      'ABCI BeginBlock',
      'ABCI EndBlock',
      'Processing governance proposal',
      'Staking rewards distributed',
      'Validator set updated',
    ];

    // Initial logs
    const initialLogs: LogEntry[] = Array.from({ length: 50 }, (_, i) => ({
      timestamp: new Date(Date.now() - (50 - i) * 1000).toISOString(),
      level: levels[Math.floor(Math.random() * levels.length)],
      message: `${messages[Math.floor(Math.random() * messages.length)]} height=${1000 + i}`,
      source: sources[Math.floor(Math.random() * sources.length)],
    }));
    setLogs(initialLogs);

    // Add new logs periodically
    const interval = setInterval(() => {
      if (!isPaused) {
        setLogs(prev => {
          const newLog: LogEntry = {
            timestamp: new Date().toISOString(),
            level: levels[Math.floor(Math.random() * levels.length)],
            message: `${messages[Math.floor(Math.random() * messages.length)]} height=${prev.length + 1000}`,
            source: sources[Math.floor(Math.random() * sources.length)],
          };
          return [...prev.slice(-499), newLog]; // Keep last 500 logs
        });
      }
    }, 1000);

    return () => clearInterval(interval);
  }, [isPaused]);

  // Auto-scroll effect
  useEffect(() => {
    if (autoScroll && logContainerRef.current) {
      logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight;
    }
  }, [logs, autoScroll]);

  // Filter and search logs
  const filteredLogs = logs.filter(log => {
    const matchesFilter = filter === 'all' || log.level === filter;
    const matchesSearch = !search ||
      log.message.toLowerCase().includes(search.toLowerCase()) ||
      log.source?.toLowerCase().includes(search.toLowerCase());
    return matchesFilter && matchesSearch;
  });

  const getLevelColor = (level: string) => {
    switch (level) {
      case 'error': return 'text-red-500 bg-red-50';
      case 'warn': return 'text-yellow-600 bg-yellow-50';
      case 'debug': return 'text-gray-500 bg-gray-50';
      default: return 'text-blue-600 bg-blue-50';
    }
  };

  const getSourceColor = (source?: string) => {
    switch (source) {
      case 'tendermint': return 'text-purple-600';
      case 'abci': return 'text-green-600';
      case 'app': return 'text-blue-600';
      case 'upgrade': return 'text-orange-600';
      default: return 'text-gray-600';
    }
  };

  const exportLogs = () => {
    const content = filteredLogs.map(log =>
      `[${log.timestamp}] [${log.level.toUpperCase()}] [${log.source || 'system'}] ${log.message}`
    ).join('\n');

    const blob = new Blob([content], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `validator-logs-${new Date().toISOString().split('T')[0]}.txt`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const clearLogs = () => {
    setLogs([]);
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
                <h1 className="text-xl font-bold text-gray-900">Validator Logs</h1>
                <p className="text-sm text-gray-500">Real-time log streaming with filtering</p>
              </div>
            </div>
            <div className="flex items-center space-x-3">
              <span className="text-sm text-gray-500">
                {filteredLogs.length} logs
              </span>
              <button onClick={exportLogs} className="btn btn-secondary text-sm">
                Export
              </button>
              <button onClick={clearLogs} className="btn btn-secondary text-sm">
                Clear
              </button>
            </div>
          </div>
        </div>
      </header>

      {/* Controls */}
      <div className="bg-white border-b border-gray-200 px-4 sm:px-6 lg:px-8 py-3">
        <div className="max-w-7xl mx-auto flex flex-wrap items-center gap-4">
          {/* Search */}
          <div className="flex-1 min-w-[200px]">
            <input
              type="text"
              placeholder="Search logs..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-purple-500 focus:border-transparent"
            />
          </div>

          {/* Level Filter */}
          <div className="flex items-center space-x-2">
            {(['all', 'info', 'warn', 'error', 'debug'] as const).map(level => (
              <button
                key={level}
                onClick={() => setFilter(level)}
                className={clsx(
                  'px-3 py-1.5 rounded-lg text-sm font-medium transition-colors',
                  filter === level
                    ? 'bg-purple-600 text-white'
                    : 'bg-gray-100 text-gray-700 hover:bg-gray-200'
                )}
              >
                {level.charAt(0).toUpperCase() + level.slice(1)}
              </button>
            ))}
          </div>

          {/* Controls */}
          <div className="flex items-center space-x-3">
            <label className="flex items-center space-x-2 text-sm text-gray-600">
              <input
                type="checkbox"
                checked={autoScroll}
                onChange={(e) => setAutoScroll(e.target.checked)}
                className="rounded text-purple-600"
              />
              <span>Auto-scroll</span>
            </label>
            <button
              onClick={() => setIsPaused(!isPaused)}
              className={clsx(
                'px-3 py-1.5 rounded-lg text-sm font-medium',
                isPaused ? 'bg-green-600 text-white' : 'bg-red-600 text-white'
              )}
            >
              {isPaused ? 'Resume' : 'Pause'}
            </button>
          </div>
        </div>
      </div>

      {/* Logs Container */}
      <main className="flex-1 overflow-hidden p-4 sm:p-6 lg:p-8">
        <div
          ref={logContainerRef}
          className="h-full bg-gray-900 rounded-lg overflow-auto font-mono text-sm"
          style={{ maxHeight: 'calc(100vh - 250px)' }}
        >
          <div className="p-4 space-y-1">
            {filteredLogs.length === 0 ? (
              <p className="text-gray-500 text-center py-8">No logs to display</p>
            ) : (
              filteredLogs.map((log, idx) => (
                <div
                  key={idx}
                  className="flex items-start space-x-3 hover:bg-gray-800 px-2 py-1 rounded"
                >
                  {/* Timestamp */}
                  <span className="text-gray-500 flex-shrink-0 w-24">
                    {new Date(log.timestamp).toLocaleTimeString()}
                  </span>

                  {/* Level Badge */}
                  <span className={clsx(
                    'px-2 py-0.5 rounded text-xs font-medium flex-shrink-0 w-14 text-center',
                    getLevelColor(log.level)
                  )}>
                    {log.level.toUpperCase()}
                  </span>

                  {/* Source */}
                  <span className={clsx(
                    'text-xs font-medium flex-shrink-0 w-20',
                    getSourceColor(log.source)
                  )}>
                    [{log.source || 'system'}]
                  </span>

                  {/* Message */}
                  <span className="text-gray-300 break-all">
                    {log.message}
                  </span>
                </div>
              ))
            )}
          </div>
        </div>
      </main>
    </div>
  );
}
