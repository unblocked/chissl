# CI & Releases

## CI Jobs
- Main test/build matrix across Linux/macOS/Windows (CGO enabled as needed)
- Unit tests exclude legacy e2e; e2e may run as non-blocking
- Release builds use goreleaser

## Windows notes
- Ensure SQLite files are closed before temp dir cleanup; tests call `Server.Shutdown()`

## Releases
- Tag with `vX.Y.Z` to trigger goreleaser pipeline

