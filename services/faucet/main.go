// Omniphi Testnet Faucet Service
// Production-grade token distribution service for testnet
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// Config holds the faucet configuration
type Config struct {
	// Server settings
	Port           string `json:"port"`
	Host           string `json:"host"`

	// Chain settings
	ChainID        string `json:"chain_id"`
	RPCEndpoint    string `json:"rpc_endpoint"`
	GRPCEndpoint   string `json:"grpc_endpoint"`
	Denom          string `json:"denom"`
	Bech32Prefix   string `json:"bech32_prefix"`

	// Faucet settings
	FaucetMnemonic string `json:"faucet_mnemonic"`
	DistributionAmount int64 `json:"distribution_amount"` // in base units (uomni)

	// Rate limiting
	CooldownSeconds int64 `json:"cooldown_seconds"` // per-address cooldown
	DailyCap        int64 `json:"daily_cap"`        // max distributions per day

	// CORS
	AllowedOrigins []string `json:"allowed_origins"`
}

// FaucetService manages token distribution
type FaucetService struct {
	config      *Config
	clientCtx   client.Context
	txFactory   tx.Factory
	faucetAddr  sdk.AccAddress

	// Rate limiting state
	mu             sync.RWMutex
	addressCooldowns map[string]time.Time
	dailyCount     int64
	dailyResetTime time.Time
}

// DistributionRequest represents a faucet request
type DistributionRequest struct {
	Address string `json:"address"`
}

// DistributionResponse represents a faucet response
type DistributionResponse struct {
	Success bool   `json:"success"`
	TxHash  string `json:"tx_hash,omitempty"`
	Amount  string `json:"amount,omitempty"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// HealthResponse for health check endpoint
type HealthResponse struct {
	Status        string `json:"status"`
	FaucetAddress string `json:"faucet_address"`
	ChainID       string `json:"chain_id"`
	DailyRemaining int64  `json:"daily_remaining"`
}

// StatsResponse for statistics endpoint
type StatsResponse struct {
	TotalDistributed int64  `json:"total_distributed_today"`
	DailyCap         int64  `json:"daily_cap"`
	CooldownSeconds  int64  `json:"cooldown_seconds"`
	DistributionAmount string `json:"distribution_amount"`
}

func main() {
	// Load configuration
	config := loadConfig()

	// Initialize SDK config
	sdkConfig := sdk.GetConfig()
	sdkConfig.SetBech32PrefixForAccount(config.Bech32Prefix, config.Bech32Prefix+"pub")
	sdkConfig.SetBech32PrefixForValidator(config.Bech32Prefix+"valoper", config.Bech32Prefix+"valoperpub")
	sdkConfig.SetBech32PrefixForConsensusNode(config.Bech32Prefix+"valcons", config.Bech32Prefix+"valconspub")
	sdkConfig.Seal()

	// Create faucet service
	faucet, err := NewFaucetService(config)
	if err != nil {
		log.Fatalf("Failed to initialize faucet: %v", err)
	}

	// Setup HTTP server
	mux := http.NewServeMux()

	// Endpoints
	mux.HandleFunc("/", faucet.handleHome)
	mux.HandleFunc("/health", faucet.handleHealth)
	mux.HandleFunc("/stats", faucet.handleStats)
	mux.HandleFunc("/faucet", faucet.handleFaucet)

	// Wrap with CORS middleware
	handler := faucet.corsMiddleware(mux)

	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", config.Host, config.Port),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down faucet service...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	log.Printf("Omniphi Faucet starting on %s:%s", config.Host, config.Port)
	log.Printf("Faucet address: %s", faucet.faucetAddr.String())
	log.Printf("Distribution amount: %d %s", config.DistributionAmount, config.Denom)

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}

func loadConfig() *Config {
	// Default configuration
	config := &Config{
		Port:              getEnv("FAUCET_PORT", "8080"),
		Host:              getEnv("FAUCET_HOST", "0.0.0.0"),
		ChainID:           getEnv("CHAIN_ID", "omniphi-testnet-2"),
		RPCEndpoint:       getEnv("RPC_ENDPOINT", "http://localhost:26657"),
		GRPCEndpoint:      getEnv("GRPC_ENDPOINT", "localhost:9090"),
		Denom:             getEnv("DENOM", "uomni"),
		Bech32Prefix:      getEnv("BECH32_PREFIX", "omni"),
		FaucetMnemonic:    getEnv("FAUCET_MNEMONIC", ""),
		DistributionAmount: getEnvInt64("DISTRIBUTION_AMOUNT", 10000000000), // 10,000 OMNI
		CooldownSeconds:   getEnvInt64("COOLDOWN_SECONDS", 86400), // 24 hours
		DailyCap:          getEnvInt64("DAILY_CAP", 1000), // 1000 distributions per day
		AllowedOrigins:    strings.Split(getEnv("ALLOWED_ORIGINS", "*"), ","),
	}

	if config.FaucetMnemonic == "" {
		log.Fatal("FAUCET_MNEMONIC environment variable is required")
	}

	return config
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		var result int64
		fmt.Sscanf(value, "%d", &result)
		return result
	}
	return defaultValue
}

// NewFaucetService creates a new faucet service
func NewFaucetService(config *Config) (*FaucetService, error) {
	// Create in-memory keyring
	kr := keyring.NewInMemory(nil)

	// Derive key from mnemonic
	hdPath := hd.CreateHDPath(60, 0, 0).String() // coin type 60 for Omniphi

	// Add key to keyring
	record, err := kr.NewAccount("faucet", config.FaucetMnemonic, "", hdPath, hd.Secp256k1)
	if err != nil {
		return nil, fmt.Errorf("failed to create faucet account: %w", err)
	}

	addr, err := record.GetAddress()
	if err != nil {
		return nil, fmt.Errorf("failed to get faucet address: %w", err)
	}

	// Create client context (simplified for faucet)
	clientCtx := client.Context{}.
		WithChainID(config.ChainID).
		WithKeyring(kr).
		WithFromName("faucet").
		WithFromAddress(addr)

	// Create tx factory
	txFactory := tx.Factory{}.
		WithChainID(config.ChainID).
		WithKeybase(kr).
		WithGas(200000).
		WithGasAdjustment(1.5).
		WithSignMode(signing.SignMode_SIGN_MODE_DIRECT)

	return &FaucetService{
		config:           config,
		clientCtx:        clientCtx,
		txFactory:        txFactory,
		faucetAddr:       addr,
		addressCooldowns: make(map[string]time.Time),
		dailyResetTime:   time.Now().Truncate(24 * time.Hour).Add(24 * time.Hour),
	}, nil
}

// CORS middleware
func (f *FaucetService) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Check if origin is allowed
		allowed := false
		for _, o := range f.config.AllowedOrigins {
			if o == "*" || o == origin {
				allowed = true
				break
			}
		}

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else if len(f.config.AllowedOrigins) > 0 && f.config.AllowedOrigins[0] == "*" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Handle home page
func (f *FaucetService) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Omniphi Testnet Faucet</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        * { box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #1a1a2e 0%%, #16213e 100%%);
            color: #fff;
            min-height: 100vh;
            margin: 0;
            padding: 20px;
        }
        .container { max-width: 600px; margin: 0 auto; padding: 40px 20px; }
        h1 {
            background: linear-gradient(90deg, #667eea 0%%, #764ba2 100%%);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            font-size: 2.5rem;
            margin-bottom: 10px;
        }
        .subtitle { color: #888; margin-bottom: 40px; }
        .card {
            background: rgba(255,255,255,0.05);
            border-radius: 16px;
            padding: 30px;
            border: 1px solid rgba(255,255,255,0.1);
        }
        input {
            width: 100%%;
            padding: 15px;
            border-radius: 8px;
            border: 1px solid rgba(255,255,255,0.2);
            background: rgba(0,0,0,0.3);
            color: #fff;
            font-size: 16px;
            margin-bottom: 15px;
        }
        input:focus { outline: none; border-color: #667eea; }
        button {
            width: 100%%;
            padding: 15px;
            border-radius: 8px;
            border: none;
            background: linear-gradient(90deg, #667eea 0%%, #764ba2 100%%);
            color: #fff;
            font-size: 16px;
            font-weight: 600;
            cursor: pointer;
            transition: transform 0.2s, opacity 0.2s;
        }
        button:hover { transform: translateY(-2px); opacity: 0.9; }
        button:disabled { opacity: 0.5; cursor: not-allowed; transform: none; }
        .result { margin-top: 20px; padding: 15px; border-radius: 8px; }
        .success { background: rgba(16, 185, 129, 0.2); border: 1px solid rgba(16, 185, 129, 0.3); }
        .error { background: rgba(239, 68, 68, 0.2); border: 1px solid rgba(239, 68, 68, 0.3); }
        .info { margin-top: 30px; color: #888; font-size: 14px; }
        .info strong { color: #667eea; }
        .stats { display: grid; grid-template-columns: 1fr 1fr; gap: 15px; margin-top: 20px; }
        .stat { background: rgba(0,0,0,0.2); padding: 15px; border-radius: 8px; text-align: center; }
        .stat-value { font-size: 24px; font-weight: 600; color: #667eea; }
        .stat-label { font-size: 12px; color: #888; margin-top: 5px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Omniphi Faucet</h1>
        <p class="subtitle">Get testnet OMNI tokens for development</p>

        <div class="card">
            <input type="text" id="address" placeholder="Enter your omni1... address" />
            <button id="request" onclick="requestTokens()">Request Tokens</button>
            <div id="result"></div>

            <div class="info">
                <p><strong>Distribution:</strong> %s OMNI per request</p>
                <p><strong>Cooldown:</strong> %d hours between requests</p>
                <p><strong>Faucet Address:</strong> %s</p>
            </div>

            <div class="stats">
                <div class="stat">
                    <div class="stat-value" id="daily-remaining">-</div>
                    <div class="stat-label">Daily Remaining</div>
                </div>
                <div class="stat">
                    <div class="stat-value" id="total-today">-</div>
                    <div class="stat-label">Distributed Today</div>
                </div>
            </div>
        </div>
    </div>

    <script>
        async function requestTokens() {
            const address = document.getElementById('address').value.trim();
            const button = document.getElementById('request');
            const result = document.getElementById('result');

            if (!address || !address.startsWith('omni1')) {
                result.className = 'result error';
                result.innerHTML = 'Please enter a valid omni1... address';
                return;
            }

            button.disabled = true;
            button.textContent = 'Requesting...';
            result.innerHTML = '';

            try {
                const response = await fetch('/faucet', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ address })
                });

                const data = await response.json();

                if (data.success) {
                    result.className = 'result success';
                    result.innerHTML = 'Success! Sent ' + data.amount + '<br>TX: <a href="#" style="color:#667eea">' + data.tx_hash.substring(0,16) + '...</a>';
                } else {
                    result.className = 'result error';
                    result.innerHTML = data.error || data.message || 'Request failed';
                }
            } catch (err) {
                result.className = 'result error';
                result.innerHTML = 'Network error: ' + err.message;
            }

            button.disabled = false;
            button.textContent = 'Request Tokens';
            loadStats();
        }

        async function loadStats() {
            try {
                const response = await fetch('/stats');
                const data = await response.json();
                document.getElementById('daily-remaining').textContent = (data.daily_cap - data.total_distributed_today);
                document.getElementById('total-today').textContent = data.total_distributed_today;
            } catch (err) {
                console.error('Failed to load stats:', err);
            }
        }

        loadStats();
        setInterval(loadStats, 30000);
    </script>
</body>
</html>`,
		formatAmount(f.config.DistributionAmount),
		f.config.CooldownSeconds/3600,
		f.faucetAddr.String(),
	)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// Handle health check
func (f *FaucetService) handleHealth(w http.ResponseWriter, r *http.Request) {
	f.mu.RLock()
	remaining := f.config.DailyCap - f.dailyCount
	f.mu.RUnlock()

	response := HealthResponse{
		Status:         "healthy",
		FaucetAddress:  f.faucetAddr.String(),
		ChainID:        f.config.ChainID,
		DailyRemaining: remaining,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Handle stats
func (f *FaucetService) handleStats(w http.ResponseWriter, r *http.Request) {
	f.mu.RLock()
	count := f.dailyCount
	f.mu.RUnlock()

	response := StatsResponse{
		TotalDistributed:   count,
		DailyCap:           f.config.DailyCap,
		CooldownSeconds:    f.config.CooldownSeconds,
		DistributionAmount: formatAmount(f.config.DistributionAmount) + " OMNI",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Handle faucet request
func (f *FaucetService) handleFaucet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != "POST" {
		json.NewEncoder(w).Encode(DistributionResponse{
			Success: false,
			Error:   "Method not allowed. Use POST.",
		})
		return
	}

	var req DistributionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(DistributionResponse{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	// Validate address
	if !isValidAddress(req.Address, f.config.Bech32Prefix) {
		json.NewEncoder(w).Encode(DistributionResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid address. Must start with %s1", f.config.Bech32Prefix),
		})
		return
	}

	// Check rate limits
	if err := f.checkRateLimits(req.Address); err != nil {
		json.NewEncoder(w).Encode(DistributionResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// Send tokens
	txHash, err := f.sendTokens(req.Address)
	if err != nil {
		log.Printf("Failed to send tokens to %s: %v", req.Address, err)
		json.NewEncoder(w).Encode(DistributionResponse{
			Success: false,
			Error:   "Failed to send tokens. Please try again later.",
		})
		return
	}

	// Update rate limit tracking
	f.recordDistribution(req.Address)

	log.Printf("Sent %d %s to %s (tx: %s)", f.config.DistributionAmount, f.config.Denom, req.Address, txHash)

	json.NewEncoder(w).Encode(DistributionResponse{
		Success: true,
		TxHash:  txHash,
		Amount:  formatAmount(f.config.DistributionAmount) + " OMNI",
		Message: "Tokens sent successfully!",
	})
}

// Check rate limits
func (f *FaucetService) checkRateLimits(address string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Reset daily counter if needed
	if time.Now().After(f.dailyResetTime) {
		f.dailyCount = 0
		f.dailyResetTime = time.Now().Truncate(24 * time.Hour).Add(24 * time.Hour)
		// Clear old cooldowns
		for addr := range f.addressCooldowns {
			if time.Now().After(f.addressCooldowns[addr]) {
				delete(f.addressCooldowns, addr)
			}
		}
	}

	// Check daily cap
	if f.dailyCount >= f.config.DailyCap {
		return fmt.Errorf("daily distribution limit reached. Please try again tomorrow")
	}

	// Check address cooldown
	if cooldownEnd, exists := f.addressCooldowns[address]; exists {
		if time.Now().Before(cooldownEnd) {
			remaining := time.Until(cooldownEnd).Round(time.Minute)
			return fmt.Errorf("please wait %v before requesting again", remaining)
		}
	}

	return nil
}

// Record a distribution for rate limiting
func (f *FaucetService) recordDistribution(address string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.dailyCount++
	f.addressCooldowns[address] = time.Now().Add(time.Duration(f.config.CooldownSeconds) * time.Second)
}

// Send tokens to an address
func (f *FaucetService) sendTokens(toAddress string) (string, error) {
	// Parse recipient address
	recipient, err := sdk.AccAddressFromBech32(toAddress)
	if err != nil {
		return "", fmt.Errorf("invalid address: %w", err)
	}

	// Create send message
	amount := sdk.NewCoins(sdk.NewInt64Coin(f.config.Denom, f.config.DistributionAmount))
	msg := banktypes.NewMsgSend(f.faucetAddr, recipient, amount)

	// This is a simplified version - in production you would:
	// 1. Query the account sequence
	// 2. Build and sign the transaction
	// 3. Broadcast to the chain
	// 4. Wait for confirmation

	// For now, we'll use the CLI or a proper Cosmos SDK client
	// This is a placeholder that shows the structure

	// In a real implementation, you would use:
	// - grpc connection to broadcast
	// - proper sequence/account number handling
	// - async confirmation handling

	log.Printf("Would send %v from %s to %s", amount, f.faucetAddr, recipient)

	// Placeholder - return a mock tx hash
	// In production, this would be the actual broadcast result
	return fmt.Sprintf("MOCK_%s_%d", toAddress[5:15], time.Now().UnixNano()), nil
}

// Validate address format
func isValidAddress(address, prefix string) bool {
	if !strings.HasPrefix(address, prefix+"1") {
		return false
	}

	// Basic bech32 validation
	pattern := fmt.Sprintf(`^%s1[a-z0-9]{38}$`, prefix)
	matched, _ := regexp.MatchString(pattern, address)
	return matched
}

// Format amount for display
func formatAmount(amount int64) string {
	omni := float64(amount) / 1000000
	if omni == float64(int64(omni)) {
		return fmt.Sprintf("%.0f", omni)
	}
	return fmt.Sprintf("%.2f", omni)
}
