# Alpha Trading - Quantitative Trading Platform

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![React](https://img.shields.io/badge/React-18+-61DAFB?style=flat&logo=react)](https://reactjs.org/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.0+-3178C6?style=flat&logo=typescript)](https://www.typescriptlang.org/)
[![License](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](LICENSE)

> A quantitative trading platform focused on crypto derivatives, supporting spot-futures arbitrage, funding rate strategies, and grid trading.

---

## Features

- **Multi-Exchange**: Trade on Binance, Bybit, OKX, Bitget, Hyperliquid, Aster DEX, Lighter
- **Multi-AI Support**: Run DeepSeek, Qwen, GPT, Claude, Gemini, Grok, Kimi
- **Strategy Studio**: Visual strategy builder with coin sources, indicators, and risk controls
- **Real-Time Dashboard**: Live positions, P/L tracking, and decision logs
- **Backtesting**: Historical simulation with performance metrics
- **Web-Based Config**: Configure everything through the web interface

> **Risk Warning**: Crypto trading carries significant risks. Use for learning/research or test with small amounts only.

---

## Supported Exchanges

### CEX (Centralized)

| Exchange | Status | Register |
|----------|--------|----------|
| **Binance** | ✅ | [Register](https://www.binance.com/join?ref=NOFXENG) |
| **Bybit** | ✅ | [Register](https://partner.bybit.com/b/83856) |
| **OKX** | ✅ | [Register](https://www.okx.com/join/1865360) |
| **Bitget** | ✅ | [Register](https://www.bitget.com/referral/register?from=referral&clacCode=c8a43172) |

### Perp-DEX (Decentralized)

| Exchange | Status | Register |
|----------|--------|----------|
| **Hyperliquid** | ✅ | [Register](https://app.hyperliquid.xyz/join/AITRADING) |
| **Aster DEX** | ✅ | [Register](https://www.asterdex.com/en/referral/fdfc0e) |
| **Lighter** | ✅ | [Register](https://app.lighter.xyz/?referral=68151432) |

---

## Supported AI Models

| AI Model | Status | Get API Key |
|----------|--------|-------------|
| **DeepSeek** | ✅ | [Get API Key](https://platform.deepseek.com) |
| **Qwen** | ✅ | [Get API Key](https://dashscope.console.aliyun.com) |
| **OpenAI (GPT)** | ✅ | [Get API Key](https://platform.openai.com) |
| **Claude** | ✅ | [Get API Key](https://console.anthropic.com) |
| **Gemini** | ✅ | [Get API Key](https://aistudio.google.com) |
| **Grok** | ✅ | [Get API Key](https://console.x.ai) |
| **Kimi** | ✅ | [Get API Key](https://platform.moonshot.cn) |

---

## Quick Start

### Docker (Recommended)

```bash
# Download and start
curl -O https://raw.githubusercontent.com/the-dev-z/alpha-trading/main/docker-compose.prod.yml
docker compose -f docker-compose.prod.yml up -d
```

Access: **http://127.0.0.1:3000**

```bash
# Management commands
docker compose -f docker-compose.prod.yml logs -f    # View logs
docker compose -f docker-compose.prod.yml restart    # Restart
docker compose -f docker-compose.prod.yml down       # Stop
docker compose -f docker-compose.prod.yml pull && docker compose -f docker-compose.prod.yml up -d  # Update
```

---

## Manual Installation

### Prerequisites

- **Go 1.21+**
- **Node.js 18+**
- **TA-Lib** (technical indicator library)

### Linux / macOS

```bash
# Install TA-Lib
# macOS
brew install ta-lib

# Ubuntu/Debian
sudo apt-get install libta-lib0-dev

# Clone and build
git clone https://github.com/the-dev-z/alpha-trading.git
cd alpha-trading

# Backend
go mod download
go build -o alpha-trading
./alpha-trading

# Frontend (new terminal)
cd web && npm install && npm run dev
```

### Windows

#### Option 1: Docker Desktop (Recommended)

1. Install [Docker Desktop](https://www.docker.com/products/docker-desktop/)
2. Open PowerShell:
   ```powershell
   curl -o docker-compose.prod.yml https://raw.githubusercontent.com/the-dev-z/alpha-trading/main/docker-compose.prod.yml
   docker compose -f docker-compose.prod.yml up -d
   ```
3. Access: **http://127.0.0.1:3000**

#### Option 2: WSL2

1. Install WSL2: `wsl --install` (PowerShell as Admin)
2. Install Ubuntu from Microsoft Store
3. In Ubuntu terminal:
   ```bash
   # Install dependencies
   sudo apt update && sudo apt upgrade -y
   wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
   sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
   echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
   source ~/.bashrc

   curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
   sudo apt-get install -y nodejs libta-lib0-dev git

   # Clone and run
   git clone https://github.com/the-dev-z/alpha-trading.git
   cd alpha-trading
   go build -o alpha-trading && ./alpha-trading
   ```

---

## Server Deployment

### Quick Deploy (HTTP)

```bash
curl -fsSL https://raw.githubusercontent.com/the-dev-z/alpha-trading/main/install.sh | bash
```

Access via `http://YOUR_SERVER_IP:3000`

### HTTPS with Cloudflare

1. Add domain to [Cloudflare](https://dash.cloudflare.com) (free plan works)
2. Create DNS A record → Your server IP (Proxied)
3. SSL/TLS → Flexible mode
4. Set `TRANSPORT_ENCRYPTION=true` in `.env`
5. Access via `https://your-domain.com`

---

## Configuration

Copy `.env.example` to `.env` and configure:

```bash
# Server ports
ALPHA_BACKEND_PORT=8080
ALPHA_FRONTEND_PORT=3000

# Security (required)
JWT_SECRET=your-random-secret-here
DATA_ENCRYPTION_KEY=your-base64-32-byte-key

# Generate keys with:
# openssl rand -base64 32
```

---

## Common Issues

### TA-Lib not found
```bash
# macOS
brew install ta-lib

# Ubuntu
sudo apt-get install libta-lib0-dev
```

### AI API timeout
- Check if API key is correct
- Check network connection
- System timeout is 120 seconds

### Frontend can't connect to backend
- Ensure backend is running on http://localhost:8080
- Check if port is occupied

---

## Documentation

| Document | Description |
|----------|-------------|
| [Architecture Overview](docs/architecture/README.md) | System design |
| [Strategy Module](docs/architecture/STRATEGY_MODULE.md) | Strategy configuration |
| [Getting Started](docs/getting-started/README.md) | Deployment guide |

---

## License

**GNU Affero General Public License v3.0 (AGPL-3.0)** - See [LICENSE](LICENSE)

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development workflow.

---

*Forked from [NOFX](https://github.com/NoFxAiOS/nofx) - AI Trading OS*
