# Code Standards

1. All exported functions must have doc comments.
2. Error messages use format: `{OPERATION}: {ROOT_CAUSE}. {RECOVERY_ACTION}`
3. Max file size: 800 lines.
4. No `fmt.Println` in production code — use structured logging.
5. HTTP handlers must validate Content-Type header.
