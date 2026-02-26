# pkg

Shared libraries used across PulseCart services.
Keep these small and stable:
- config (env loading)
- logx (structured logging)
- httpx (common middleware, health handlers)
- dbx (postgres helpers)
- redix (redis helpers)
- natsx (nats helpers)
