# Alpha Trading - Quantitative Trading Platform

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![React](https://img.shields.io/badge/React-18+-61DAFB?style=flat&logo=react)](https://reactjs.org/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.0+-3178C6?style=flat&logo=typescript)](https://www.typescriptlang.org/)
[![License](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](LICENSE)

> A quantitative trading platform focused on crypto derivatives, supporting spot-futures arbitrage, funding rate strategies, and grid trading.

---

## Features

- **Multi-Exchange**: Trade on Binance, Bybit, OKX, Bitget, Hyperliquid, Aster DEX, Lighter
- **Strategy Studio**: Visual strategy builder with coin sources, indicators, and risk controls
- **Real-Time Dashboard**: Live positions, P/L tracking, and decision logs
- **Backtesting**: Historical simulation with performance metrics
- **Web-Based Config**: Configure everything through the web interface

> **Risk Warning**: Crypto trading carries significant risks. Use for learning/research or test with small amounts only.

---

## Quick Start

### Docker (Recommended)

```bash
# Download and start
curl -O https://raw.githubusercontent.com/the-dev-z/alpha-trading/main/docker-compose.prod.yml
docker compose -f docker-compose.prod.yml up -d
```

Access: **http://127.0.0.1:3000**

### Manual Installation

#### Prerequisites

- **Go 1.21+**
- **Node.js 18+**
- **TA-Lib**: `brew install ta-lib` (macOS) or `apt-get install libta-lib0-dev` (Ubuntu)

#### Steps

```bash
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

Access: **http://127.0.0.1:3000**

---

## Supported Exchanges

| Exchange | Type | Status |
|----------|------|--------|
| **Binance** | CEX | ✅ |
| **Bybit** | CEX | ✅ |
| **OKX** | CEX | ✅ |
| **Bitget** | CEX | ✅ |
| **Hyperliquid** | Perp-DEX | ✅ |
| **Aster DEX** | Perp-DEX | ✅ |
| **Lighter** | Perp-DEX | ✅ |

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
```

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
