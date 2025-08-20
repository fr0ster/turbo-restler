# Changelog

All notable changes to this project will be documented in this file.

## [v0.14.25] - 2024-08-20

### Added
- **Structured Error Handling**: New `WebSocketError` type with `Op`, `Message`, `Err`, and `Code` fields
- **Enhanced Logging**: Improved `LogRecord` with `LogLevel` (Debug, Info, Warn, Error), `Time`, and `Context`
- **WebSocket Metrics**: `WebSocketMetrics` struct for tracking messages, bytes, errors, reconnects, and uptime
- **Circuit Breaker Pattern**: REST API circuit breaker with configurable failure thresholds and recovery timeouts
- **Automatic Reconnection**: `ReconnectConfig` with exponential backoff and retry limits
- **Comprehensive Makefile**: Build automation with targets for build, test, clean, lint, and format
- **Quiet Test Mode**: `make test-quiet` to filter out WebSocket noise in tests
- **Examples**: Working WebSocket example with configuration
- **Enhanced Configuration**: `WebSocketConfig` and `RestAPIConfig` with detailed settings

### Changed
- **WebSocket Wrapper**: Enhanced with metrics, better error handling, and configuration options
- **REST API Client**: Added retry logic, circuit breaker, and improved error handling
- **Test Servers**: Improved error handling for "broken pipe" scenarios (no more noisy logs)
- **Backward Compatibility**: All existing APIs continue to work unchanged

### Fixed
- **Atomic Bool Issue**: Fixed `sync/atomic.Bool` copying problem in `WrapServerConn`
- **Test Noise**: Eliminated distracting "broken pipe" error messages in tests
- **Circuit Breaker State**: Fixed state persistence between API calls

### Technical Details
- **Go Version**: 1.23.0+
- **Dependencies**: Updated to latest stable versions
- **Build System**: Binaries now compile to `bin/` directory (gitignored)
- **Testing**: Added race detection and coverage reporting options

## [v0.14.24] - Previous Version
- Add assertReceived utility function for message order verification in WebSocket tests
- Refactor TestServerWrapper_ConcurrentMessages with improved message handling
