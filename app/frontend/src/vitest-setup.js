// SPDX-License-Identifier: GPL-3.0-or-later

// Extends vitest's `expect` with jest-dom's DOM matchers (toBeDisabled,
// toBeInTheDocument, etc) for component tests. @testing-library/svelte's
// own vite plugin (see vite.config.js) adds its OWN setup file (auto
// cleanup) after this one — both run.
import '@testing-library/jest-dom/vitest'
