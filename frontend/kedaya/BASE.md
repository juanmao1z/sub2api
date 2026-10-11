# Kedaya Review Base

`frontend/kedaya/base/` is the synchronized compiled UI base copied from `kedaya-review/public/`.

The active frontend build does not use the legacy Vue source tree. `kedaya/assemble.mjs` validates the base asset hashes, applies only the Sub2API production contract adapters, and writes the embedded frontend to `backend/internal/web/dist/`.

The isolated review server, fixtures, review session, synthetic data, screenshots, and evidence remain outside the production project. The original tracked `sub2api-custom` source is preserved in `archives/`.

## Commands

```bash
pnpm run build
pnpm run typecheck
pnpm run dev
```

`pnpm run dev` assembles the base and serves it locally. Set `VITE_DEV_PROXY_TARGET` to proxy API requests to a running Sub2API backend.
