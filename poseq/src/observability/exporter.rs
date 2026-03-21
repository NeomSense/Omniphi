//! HTTP metrics + status exporter for PoSeq devnet operation.
//!
//! Endpoints:
//! - `GET /metrics`  — Prometheus text-format metrics
//! - `GET /healthz`  — liveness probe, returns "ok\n"
//! - `GET /status`   — JSON node status snapshot (epoch, slot, peers, exports, snapshots)
//!
//! # Usage
//! ```rust,no_run
//! use omniphi_poseq::observability::metrics::PoSeqMetrics;
//! use omniphi_poseq::observability::exporter::MetricsExporter;
//!
//! let metrics = PoSeqMetrics::new().unwrap();
//! let exporter = MetricsExporter::new(metrics.clone());
//! tokio::spawn(async move { exporter.serve("0.0.0.0:9090").await; });
//! ```

use std::sync::Arc;

use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::TcpListener;
use tokio::sync::Mutex;

use crate::observability::metrics::PoSeqMetrics;
use crate::networking::node_runner::NodeState;

// hex is a workspace dependency available in all crate members
extern crate hex;

/// Lightweight JSON status snapshot returned by `GET /status`.
/// Intentionally flat and human-readable for devnet operations.
#[derive(serde::Serialize)]
pub struct NodeStatusJson {
    pub node_id_prefix: String,
    pub ready: bool,
    pub current_epoch: u64,
    pub current_slot: u64,
    pub in_committee: bool,
    pub latest_snapshot_epoch: Option<u64>,
    pub exported_epoch_count: usize,
    pub exported_epochs: Vec<u64>,
    pub latest_finalized: Option<String>,
    pub slog_total: u64,
    pub peer_count: usize,
    pub sync_status: Option<crate::sync::SyncStatus>,
}

/// Serves Prometheus `/metrics`, `/healthz`, and `/status` over plain HTTP.
///
/// This is intentionally minimal (no framework dependency).  Use a reverse proxy
/// (nginx, caddy) in production to add TLS.
pub struct MetricsExporter {
    metrics: Arc<PoSeqMetrics>,
    /// Optional reference to node state for the /status endpoint.
    /// When None, /status returns a stub.
    state: Option<Arc<Mutex<NodeState>>>,
    node_id_prefix: String,
}

impl MetricsExporter {
    pub fn new(metrics: PoSeqMetrics) -> Self {
        MetricsExporter {
            metrics: Arc::new(metrics),
            state: None,
            node_id_prefix: "unknown".to_string(),
        }
    }

    pub fn from_arc(metrics: Arc<PoSeqMetrics>) -> Self {
        MetricsExporter {
            metrics,
            state: None,
            node_id_prefix: "unknown".to_string(),
        }
    }

    /// Attach live node state for the `/status` endpoint.
    pub fn with_state(mut self, state: Arc<Mutex<NodeState>>, node_id_prefix: String) -> Self {
        self.state = Some(state);
        self.node_id_prefix = node_id_prefix;
        self
    }

    /// Build a `NodeStatusJson` snapshot from the current node state.
    async fn build_status(&self) -> NodeStatusJson {
        if let Some(ref state_ref) = self.state {
            let s = state_ref.lock().await;
            NodeStatusJson {
                node_id_prefix: self.node_id_prefix.clone(),
                ready: true,
                current_epoch: s.current_epoch,
                current_slot: s.current_slot,
                in_committee: s.in_committee,
                latest_snapshot_epoch: s.latest_snapshot_epoch,
                exported_epoch_count: s.exported_epochs.len(),
                exported_epochs: s.exported_epochs.iter().copied().collect(),
                latest_finalized: s.latest_finalized.map(|id| hex::encode(id)),
                slog_total: s.slog.total_appended,
                peer_count: s.connected_peers,
                sync_status: Some(s.sync_engine.sync_status()),
            }
        } else {
            NodeStatusJson {
                node_id_prefix: self.node_id_prefix.clone(),
                ready: false,
                current_epoch: 0,
                current_slot: 0,
                in_committee: false,
                latest_snapshot_epoch: None,
                exported_epoch_count: 0,
                exported_epochs: vec![],
                latest_finalized: None,
                slog_total: 0,
                peer_count: 0,
                sync_status: None,
            }
        }
    }

    /// Start listening and serving metrics.  Runs forever.
    pub async fn serve(&self, addr: &str) {
        let listener = match TcpListener::bind(addr).await {
            Ok(l) => l,
            Err(e) => {
                eprintln!("[metrics] Failed to bind {addr}: {e}");
                return;
            }
        };
        println!("[metrics] Serving at http://{addr}/metrics  /healthz  /status");

        loop {
            match listener.accept().await {
                Err(e) => {
                    eprintln!("[metrics] Accept error: {e}");
                    continue;
                }
                Ok((mut stream, _peer)) => {
                    let metrics = Arc::clone(&self.metrics);
                    let status_json = self.build_status().await;

                    tokio::spawn(async move {
                        // Read the request (drain enough to identify the path)
                        let mut buf = [0u8; 1024];
                        let n = stream.read(&mut buf).await.unwrap_or(0);
                        let request = String::from_utf8_lossy(&buf[..n]);

                        let (status, content_type, body) =
                            if request.starts_with("GET /metrics") {
                                let body = metrics.render();
                                ("200 OK", "text/plain; version=0.0.4; charset=utf-8", body)
                            } else if request.starts_with("GET /healthz") || request.starts_with("GET /health") {
                                ("200 OK", "text/plain", "ok\n".to_string())
                            } else if request.starts_with("GET /status") {
                                let body = serde_json::to_string_pretty(&status_json)
                                    .unwrap_or_else(|_| r#"{"error":"serialization failed"}"#.to_string());
                                ("200 OK", "application/json", body)
                            } else {
                                ("404 Not Found", "text/plain", "not found\n".to_string())
                            };

                        let response = format!(
                            "HTTP/1.1 {status}\r\nContent-Type: {content_type}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                            body.len()
                        );
                        let _ = stream.write_all(response.as_bytes()).await;
                    });
                }
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::observability::metrics::PoSeqMetrics;
    use tokio::io::AsyncWriteExt;

    #[tokio::test]
    async fn test_metrics_endpoint_returns_200() {
        let metrics = PoSeqMetrics::new().unwrap();
        metrics.batches_finalized.inc();
        let exporter = MetricsExporter::new(metrics);

        // Bind to an ephemeral port
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let addr = listener.local_addr().unwrap().to_string();
        drop(listener);

        let exporter = Arc::new(exporter);
        let exporter_clone = Arc::clone(&exporter);
        let addr_clone = addr.clone();
        tokio::spawn(async move {
            exporter_clone.serve(&addr_clone).await;
        });

        // Give the server a moment to start
        tokio::time::sleep(tokio::time::Duration::from_millis(50)).await;

        // Connect and request metrics
        let mut stream = tokio::net::TcpStream::connect(&addr).await.unwrap();
        stream.write_all(b"GET /metrics HTTP/1.0\r\nHost: localhost\r\n\r\n").await.unwrap();

        let mut buf = Vec::new();
        let mut tmp = [0u8; 4096];
        loop {
            match tokio::time::timeout(
                tokio::time::Duration::from_millis(500),
                stream.read(&mut tmp),
            ).await {
                Ok(Ok(0)) | Err(_) => break,
                Ok(Ok(n)) => buf.extend_from_slice(&tmp[..n]),
                Ok(Err(_)) => break,
            }
        }
        let response = String::from_utf8_lossy(&buf);
        assert!(response.contains("200 OK"), "Expected 200 OK, got: {}", &response[..response.len().min(200)]);
        assert!(response.contains("poseq_batches_finalized_total 1"));
    }

    #[tokio::test]
    async fn test_health_endpoint() {
        let metrics = PoSeqMetrics::new().unwrap();
        let exporter = Arc::new(MetricsExporter::new(metrics));
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let addr = listener.local_addr().unwrap().to_string();
        drop(listener);

        let ec = Arc::clone(&exporter);
        let ac = addr.clone();
        tokio::spawn(async move { ec.serve(&ac).await; });
        tokio::time::sleep(tokio::time::Duration::from_millis(50)).await;

        let mut stream = tokio::net::TcpStream::connect(&addr).await.unwrap();
        stream.write_all(b"GET /healthz HTTP/1.0\r\nHost: localhost\r\n\r\n").await.unwrap();

        let mut buf = vec![0u8; 512];
        let n = tokio::time::timeout(
            tokio::time::Duration::from_millis(500),
            stream.read(&mut buf),
        ).await.unwrap_or(Ok(0)).unwrap_or(0);
        let response = String::from_utf8_lossy(&buf[..n]);
        assert!(response.contains("200 OK"));
    }
}
