# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

QuickEx Go is a unified cryptocurrency trading library providing consistent interfaces for spot and futures trading across multiple exchanges (Binance, Bybit, KuCoin, Upbit). The architecture emphasizes interface standardization, decimal precision for financial calculations, and automatic credential management.

## Build and Development Commands

```bash
# Build the project
go build ./...

# Run tests (requires test_config.toml with API credentials)
go test ./client -v

# Run specific test
go test -run TestSubscribeQuotes ./client -v

# Test specific exchange
go test -run "TestOrderScenarios/kucoin" ./client -v

# Install dependencies
go mod download

# Update dependencies
go mod tidy

# Verify module dependencies
go mod verify
```

## High-Level Architecture

### Core Interfaces (core/interface.go)
- `PublicClient`: Market data operations (quotes, orderbooks, market rules)
- `PrivateClient`: Trading operations (orders, balances, connection management)
- `TransactionClient`: Withdrawal/deposit operations

### Exchange Implementations
Each exchange follows a consistent structure:
```
client/
├── {exchange}/
│   ├── client.go          # Main client implementation
│   ├── market.go          # Public market data methods
│   ├── order.go           # Private trading methods
│   ├── websocket.go       # WebSocket connections
│   └── futures/           # Futures-specific implementations
│       ├── client.go
│       ├── market.go
│       ├── order.go
│       └── websocket_connection.go
```

### Key Design Patterns

1. **Factory Pattern** (client/factory.go):
   - `NewPrivateClient()` creates spot clients
   - `NewFuturesPrivateClient()` creates futures clients
   - Automatic credential detection (hex string vs PEM file)

2. **WebSocket Architecture**:
   - Each subscription creates independent WebSocket connection
   - Channel-based quote distribution with buffering (size 100)
   - Graceful shutdown via context cancellation
   - Race condition prevention with closed flags

3. **Decimal Precision**:
   - All monetary values use `shopspring/decimal`
   - No float operations for financial calculations
   - Consistent precision handling across exchanges

4. **Error Handling**:
   - Unified error types across exchanges
   - Error callback handlers for async operations
   - Channel overflow protection with dropping strategy

## Critical Implementation Details

### WebSocket Connection Management
- **Issue**: Channel close race conditions when multiple symbols subscribed
- **Solution**: Added `closed` flag with mutex protection before channel operations
- **Pattern**: Check closed state before sending quotes to prevent panics

### Exchange-Specific Quirks
- **KuCoin Futures**: Requires individual symbol subscriptions (not batch)
- **KuCoin Authentication**: Uses passphrase signature for private WebSocket
- **Bybit**: Local fork in go.mod replace directive
- **Binance**: Supports both production and testnet endpoints

### Testing Infrastructure
- Tests use `test_config.toml` (gitignored) for credentials
- Test factory loads clients dynamically based on config
- Performance tracking built into test framework
- Concurrent testing of multiple exchanges supported

## Documentation Guidelines

### IMPORTANT: Single README Policy
- **DO NOT** create additional documentation files beyond README.md in subdirectories
- All documentation should be consolidated into the existing README.md file
- If you need to document something new, update the relevant README.md instead of creating new files
- This keeps documentation centralized and easier to maintain

## Common Development Tasks

### Adding New Exchange
1. Create directory structure: `client/{exchange}/` and `client/{exchange}/futures/`
2. Implement core interfaces from `core/interface.go`
3. Add exchange constants to `client/factory.go`
4. Update factory methods to instantiate new client
5. Add test configuration to `test_config.toml.example`

### Debugging WebSocket Issues
1. Check channel buffer size (default 100)
2. Verify closed flag implementation in connection handler
3. Monitor error callbacks for "channel full" messages
4. Ensure proper context cancellation in cleanup

### Running Integration Tests
```bash
# Create test_config.toml from example
cp client/test_config.toml.example client/test_config.toml
# Edit with your API credentials
# Run tests
cd client && go test -v
```

### Testing Guidelines
- **NEVER create mock tests** - All tests should use real API credentials from test_config.toml
- The project has all necessary credentials configured for testing
- Focus on integration tests that verify actual exchange behavior
- Mock tests hide real API behaviors and should be avoided

# important-instruction-reminders
Do what has been asked; nothing more, nothing less.
NEVER create files unless they're absolutely necessary for achieving your goal.
ALWAYS prefer editing an existing file to creating a new one.
NEVER proactively create documentation files (*.md) or README files. Only create documentation files if explicitly requested by the User.