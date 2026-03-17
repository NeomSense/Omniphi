//! HTTP metrics exporter for Prometheus scraping.
//!
//! Serves a `/metrics` endpoint on a configurable address.
//! All PoSeq metrics are rendered in Prometheus text format.
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

use crate::observability::metrics::PoSeqMetrics;

/// Serves a Prometheus `/metrics` endpoint over plain HTTP.
///
/// This is intentionally minimal (no framework dependency) — it handles only
/// `GET /metrics` and returns 404 for everything else.  Use a reverse proxy
/// (nginx, caddy) in production to add TLS.
pub struct MetricsExporter {
    metrics: Arc<PoSeqMetrics>,
}

impl MetricsExporter {
    pub fn new(metrics: PoSeqMetrics) -> Self {
        MetricsExporter { metrics: Arc::new(metrics) }
    }

    pub fn from_arc(metrics: Arc<PoSeqMetrics>) -> Self {
        MetricsExporter { metrics }
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
        println!("[metrics] Serving Prometheus metrics at http://{addr}/metrics");

        loop {
            match listener.accept().await {
                Err(e) => {
                    eprintln!("[metrics] Accept error: {e}");
                    continue;
                }
                Ok((mut stream, _peer)) => {
                    let metrics = Arc::clone(&self.metrics);
                    tokio::spawn(async move {
                        // Read the request (we don't parse it carefully — just drain enough)
                        let mut buf = [0u8; 1024];
                        let n = stream.read(&mut buf).await.unwrap_or(0);
                        let request = String::from_utf8_lossy(&buf[..n]);

                        let (status, body) = if request.starts_with("GET /metrics") {
                            let body = metrics.render();
                            ("200 OK", body)
                        } else if request.starts_with("GET /healthz") || request.starts_with("GET /health") {
                            ("200 OK", "ok\n".to_string())
                        } else {
                            ("404 Not Found", "not found\n".to_string())
                        };

                        let response = format!(
                            "HTTP/1.1 {status}\r\nContent-Type: text/plain; version=0.0.4; charset=utf-8\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
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
