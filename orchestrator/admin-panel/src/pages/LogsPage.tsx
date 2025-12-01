/**
 * Real-time Logs Viewer Page
 */

import { useEffect, useState, useRef, useCallback } from 'react';
import { format } from 'date-fns';
import {
  RefreshCw,
  Pause,
  Play,
  Download,
  Trash2,
  ArrowDown,
  Search,
} from 'lucide-react';
import { api } from '../services/api';
import type { LogEntry, LogLevel, LogSource } from '../types';

export function LogsPage() {
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [streaming, setStreaming] = useState(false);
  const [paused, setPaused] = useState(false);
  const [autoScroll, setAutoScroll] = useState(true);
  const [levelFilter, setLevelFilter] = useState<LogLevel | ''>('');
  const [sourceFilter, setSourceFilter] = useState<LogSource | ''>('');
  const [searchQuery, setSearchQuery] = useState('');

  const logsEndRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const eventSourceRef = useRef<EventSource | null>(null);

  // Fetch initial logs
  const fetchLogs = useCallback(async () => {
    setLoading(true);
    const result = await api.logs.list({
      source: sourceFilter || undefined,
      level: levelFilter || undefined,
      limit: 200,
    });
    if (result.success && result.data) {
      setLogs(result.data);
    }
    setLoading(false);
  }, [levelFilter, sourceFilter]);

  useEffect(() => {
    fetchLogs();
  }, [fetchLogs]);

  // Auto scroll to bottom
  useEffect(() => {
    if (autoScroll && logsEndRef.current) {
      logsEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [logs, autoScroll]);

  // Start streaming
  const startStreaming = () => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
    }

    const es = api.logs.stream((log) => {
      if (!paused) {
        setLogs((prev) => [...prev.slice(-499), log]);
      }
    });

    if (es) {
      eventSourceRef.current = es;
      setStreaming(true);
    }
  };

  // Stop streaming
  const stopStreaming = () => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
    setStreaming(false);
  };

  // Toggle pause
  const togglePause = () => {
    setPaused(!paused);
  };

  // Clear logs
  const clearLogs = () => {
    setLogs([]);
  };

  // Download logs
  const downloadLogs = async () => {
    const blob = await api.logs.download({
      source: sourceFilter || undefined,
      level: levelFilter || undefined,
    });

    if (blob) {
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `orchestrator-logs-${format(new Date(), 'yyyy-MM-dd-HHmmss')}.log`;
      a.click();
      URL.revokeObjectURL(url);
    } else {
      // Fallback: download current logs as JSON
      const content = JSON.stringify(logs, null, 2);
      const fallbackBlob = new Blob([content], { type: 'application/json' });
      const url = URL.createObjectURL(fallbackBlob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `orchestrator-logs-${format(new Date(), 'yyyy-MM-dd-HHmmss')}.json`;
      a.click();
      URL.revokeObjectURL(url);
    }
  };

  // Filter logs
  const filteredLogs = logs.filter((log) => {
    if (levelFilter && log.level !== levelFilter) return false;
    if (sourceFilter && log.source !== sourceFilter) return false;
    if (searchQuery) {
      const query = searchQuery.toLowerCase();
      return (
        log.message.toLowerCase().includes(query) ||
        log.source.toLowerCase().includes(query) ||
        log.request_id?.toLowerCase().includes(query) ||
        log.node_id?.toLowerCase().includes(query)
      );
    }
    return true;
  });

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
      }
    };
  }, []);

  const levelColors: Record<LogLevel, string> = {
    debug: 'text-gray-400',
    info: 'text-blue-400',
    warn: 'text-yellow-400',
    error: 'text-red-400',
  };

  const sourceColors: Record<LogSource, string> = {
    orchestrator: 'bg-purple-900/30 text-purple-400',
    provisioning: 'bg-blue-900/30 text-blue-400',
    health: 'bg-green-900/30 text-green-400',
    docker: 'bg-cyan-900/30 text-cyan-400',
    chain: 'bg-orange-900/30 text-orange-400',
  };

  return (
    <div className="p-8 h-screen flex flex-col">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white">Logs Viewer</h1>
          <p className="text-dark-400 mt-1">Real-time orchestrator and node logs</p>
        </div>
        <div className="flex items-center space-x-2">
          {streaming ? (
            <button onClick={stopStreaming} className="btn btn-danger">
              <Pause className="w-4 h-4 mr-2" />
              Stop Streaming
            </button>
          ) : (
            <button onClick={startStreaming} className="btn btn-success">
              <Play className="w-4 h-4 mr-2" />
              Start Streaming
            </button>
          )}
          <button onClick={fetchLogs} className="btn btn-secondary">
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
          </button>
        </div>
      </div>

      {/* Filters */}
      <div className="card mb-4 p-4">
        <div className="flex flex-wrap items-center gap-4">
          <div className="flex-1 min-w-[200px]">
            <div className="relative">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-dark-400" />
              <input
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder="Search logs..."
                className="input pl-10 py-2"
              />
            </div>
          </div>

          <select
            value={levelFilter}
            onChange={(e) => setLevelFilter(e.target.value as LogLevel | '')}
            className="select w-32"
          >
            <option value="">All Levels</option>
            <option value="debug">Debug</option>
            <option value="info">Info</option>
            <option value="warn">Warning</option>
            <option value="error">Error</option>
          </select>

          <select
            value={sourceFilter}
            onChange={(e) => setSourceFilter(e.target.value as LogSource | '')}
            className="select w-40"
          >
            <option value="">All Sources</option>
            <option value="orchestrator">Orchestrator</option>
            <option value="provisioning">Provisioning</option>
            <option value="health">Health</option>
            <option value="docker">Docker</option>
            <option value="chain">Chain</option>
          </select>

          <div className="flex items-center space-x-2">
            <button
              onClick={togglePause}
              disabled={!streaming}
              className={`btn btn-sm ${paused ? 'btn-success' : 'btn-secondary'}`}
              title={paused ? 'Resume' : 'Pause'}
            >
              {paused ? <Play className="w-4 h-4" /> : <Pause className="w-4 h-4" />}
            </button>

            <button
              onClick={() => setAutoScroll(!autoScroll)}
              className={`btn btn-sm ${autoScroll ? 'btn-primary' : 'btn-secondary'}`}
              title="Auto-scroll"
            >
              <ArrowDown className="w-4 h-4" />
            </button>

            <button onClick={downloadLogs} className="btn btn-secondary btn-sm" title="Download">
              <Download className="w-4 h-4" />
            </button>

            <button onClick={clearLogs} className="btn btn-secondary btn-sm" title="Clear">
              <Trash2 className="w-4 h-4" />
            </button>
          </div>
        </div>
      </div>

      {/* Stats */}
      <div className="flex items-center justify-between mb-4 text-sm">
        <div className="flex items-center space-x-4">
          <span className="text-dark-400">
            {filteredLogs.length} logs {searchQuery || levelFilter || sourceFilter ? '(filtered)' : ''}
          </span>
          {streaming && (
            <span className="flex items-center text-green-400">
              <span className="w-2 h-2 bg-green-500 rounded-full mr-2 animate-pulse" />
              Streaming {paused && '(Paused)'}
            </span>
          )}
        </div>
        <div className="flex items-center space-x-4">
          <span className="text-dark-500">
            Errors: <span className="text-red-400">{logs.filter(l => l.level === 'error').length}</span>
          </span>
          <span className="text-dark-500">
            Warnings: <span className="text-yellow-400">{logs.filter(l => l.level === 'warn').length}</span>
          </span>
        </div>
      </div>

      {/* Logs Container */}
      <div
        ref={containerRef}
        className="log-container flex-1 overflow-y-auto"
      >
        {loading ? (
          <div className="flex items-center justify-center h-32">
            <RefreshCw className="w-6 h-6 text-omniphi-500 animate-spin" />
          </div>
        ) : filteredLogs.length === 0 ? (
          <div className="flex items-center justify-center h-32 text-dark-400">
            No logs to display
          </div>
        ) : (
          <div className="divide-y divide-dark-800">
            {filteredLogs.map((log) => (
              <div key={log.id} className="log-entry flex items-start space-x-3 py-2">
                {/* Timestamp */}
                <span className="log-timestamp flex-shrink-0 w-32">
                  {format(new Date(log.timestamp), 'HH:mm:ss.SSS')}
                </span>

                {/* Level */}
                <span className={`log-level flex-shrink-0 w-12 uppercase text-xs font-bold ${levelColors[log.level]}`}>
                  {log.level}
                </span>

                {/* Source */}
                <span className={`flex-shrink-0 px-2 py-0.5 rounded text-xs ${sourceColors[log.source]}`}>
                  {log.source}
                </span>

                {/* Message */}
                <span className="log-message flex-1 break-all">
                  {log.message}
                  {(log.request_id || log.node_id) && (
                    <span className="text-dark-500 ml-2 text-xs">
                      {log.request_id && `[req:${log.request_id}]`}
                      {log.node_id && `[node:${log.node_id}]`}
                    </span>
                  )}
                </span>
              </div>
            ))}
            <div ref={logsEndRef} />
          </div>
        )}
      </div>
    </div>
  );
}

export default LogsPage;
