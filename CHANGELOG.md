# Changelog

## [1.1.1](https://github.com/enzoamate/oklavier/compare/v1.1.0...v1.1.1) (2026-05-06)


### Bug Fixes

* **ui:** harden session card drag against browser gesture hijack ([75f5e49](https://github.com/enzoamate/oklavier/commit/75f5e495d3d797b26ba8f05e2f152e50acaf33f2))

## [1.1.0](https://github.com/enzoamate/oklavier/compare/v1.0.3...v1.1.0) (2026-05-06)


### Features

* **helm:** make front Service.type configurable + wire FRONTEND_URL/ALLOWED_ORIGINS env ([42642e6](https://github.com/enzoamate/oklavier/commit/42642e617256a88840b85bd6ea722f5160a10dbc))
* **security:** NetworkPolicy + gosec CI + structured logs + retention + quota lock + mTLS ([130427f](https://github.com/enzoamate/oklavier/commit/130427f2917bc4152d1f86b10cd5050a55af26df))
* **ui:** draggable + minimizable active session cards ([597708e](https://github.com/enzoamate/oklavier/commit/597708e55299c2bd80ebaac255b04d15c2299558))
* **ui:** drop unused 'copy link' button from session card ([4dda84d](https://github.com/enzoamate/oklavier/commit/4dda84d3111ef515e03414f6a2f23246366560f4))


### Bug Fixes

* **ci:** bump helm smoke-render values past the new length floors ([1210698](https://github.com/enzoamate/oklavier/commit/1210698df71826880018f5b9db1c7ab4dee98619))
* **ci:** gosec excludes false-positive taint rules + JSON log latency type ([7c4ea38](https://github.com/enzoamate/oklavier/commit/7c4ea383f3dfa43f8ad7668b1d8bc9e177f8ce33))
* **frontend:** callback page Suspense + clean obsolete ts-expect-error ([1ee053f](https://github.com/enzoamate/oklavier/commit/1ee053faa5e83760d391e2d8d2723fefc7ba370a))
* **frontend:** CSRF middleware allowlist uses Host header + FRONTEND_URL ([8a11d58](https://github.com/enzoamate/oklavier/commit/8a11d580a916e3a1b35d6cee101803a125252c91))
* **helm:** RWO recordings PVC + Recreate strategy on agent/guacd ([90bd3b4](https://github.com/enzoamate/oklavier/commit/90bd3b4f0dbe13575c565a5b72c2fdc8eda46a0a))
* **security:** close ZAP baseline cosmetic findings ([0609771](https://github.com/enzoamate/oklavier/commit/0609771f88ae1d4ac5c22895fc0dd03ed38151fd))
* **security:** comprehensive hardening pass — auth/IDOR/RCE/SSRF/crypto ([a08b7b7](https://github.com/enzoamate/oklavier/commit/a08b7b7ed0a822653661479d6c96609b4509eff9))
* **security:** CSRF guard at the BFF, not just the Go API ([286bcda](https://github.com/enzoamate/oklavier/commit/286bcda390cc9e9b189991959a0bc504b7f81e7f))
* **security:** CSRFGuard exempts X-Agent-Token (S2S auth) ([f1b392c](https://github.com/enzoamate/oklavier/commit/f1b392c4a210affe7f908d421bff689e5feeb5e0))
* **security:** drop legacy SHA-256 password code + bump Fiber + cert-validation false positive ([e65f981](https://github.com/enzoamate/oklavier/commit/e65f981a22796fa6d384a5257337fa6bc34b62b7))
* **security:** KC-4 — auth tokens to httpOnly cookies + CSRF guard ([cdb7214](https://github.com/enzoamate/oklavier/commit/cdb7214da6b2f7b4a300b1544791cc01e3c60f55))
* **security:** minor hardening — mountPath prefix match + screenshot needs bearer ([7ce98e6](https://github.com/enzoamate/oklavier/commit/7ce98e6cf6c75f89af5818163cc2d4538adb29b1))
* **security:** OIDC callback cookie names match the middleware ([b7cd2d2](https://github.com/enzoamate/oklavier/commit/b7cd2d2b569f877e679c5d779f77c9807383fbca))
* **security:** proxy dashboard screenshot through the core (no bearer in browser) ([6efcec0](https://github.com/enzoamate/oklavier/commit/6efcec0ca984887afcb4283aa24a0eccb9f1ed28))
* **security:** widen InternalOnly bypass for browser-public OIDC + branding paths ([fc9e74b](https://github.com/enzoamate/oklavier/commit/fc9e74b96217efc1d6b7563c2aa1669e5305b438))
* **ui:** align SessionLike to WorkspaceSession exactly (verified locally) ([ec6d860](https://github.com/enzoamate/oklavier/commit/ec6d8600dd75288ba132191891cb91e9e2024653))
* **ui:** cleaner session-launch overlay for server (RDP/VNC) workspaces ([24b65d0](https://github.com/enzoamate/oklavier/commit/24b65d05aba8fb501beb7f13aae562a99148a047))
* **ui:** drop the index signature on SessionLike (third tsc pass) ([bf7c633](https://github.com/enzoamate/oklavier/commit/bf7c633f346cb71ddadc5259e212e4179b2d6465))
* **ui:** drop unused ReactNode import and use const for snap y ([f9a78bd](https://github.com/enzoamate/oklavier/commit/f9a78bd842046eccb7900afa1ffb2f4c1caf35e0))
* **ui:** narrow draggable-card date types to string ([750cabd](https://github.com/enzoamate/oklavier/commit/750cabdcb425ff15b519ba0605a10b0075c14b6d))
* **ui:** re-mint short-lived screenshot bearer in /api/sessions list ([e64dc89](https://github.com/enzoamate/oklavier/commit/e64dc893d05b21d7c030205a946097fef3ee45c5))
* **ui:** widen SessionLike to accept the parent's WorkspaceSession shape ([a2f987d](https://github.com/enzoamate/oklavier/commit/a2f987de9daeec7e7e911a2f3d822b351af3dfa4))
* **viewer:** self-heal display resize after connect ([525c8b1](https://github.com/enzoamate/oklavier/commit/525c8b15747c1a8ab80d3704aa662349d0393ce9))
