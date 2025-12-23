import { afterEach } from 'vitest';
import { cleanup } from '@testing-library/react';
import '@testing-library/jest-dom/vitest';

// =============================================================================
// TEST COVERAGE GUIDELINES
// =============================================================================
// Current coverage: 23 test files (~18% file ratio) focused on critical paths.
// This is acceptable given Storybook stories provide visual regression testing.
//
// Priority areas for new tests:
// 1. Hooks with complex logic (useSmartPolling, useSessionFilters, useLoadSession)
// 2. Services with API/parsing logic (api.ts, transcriptService.ts)
// 3. Utility functions with edge cases (formatting, validation)
//
// Lower priority (covered by Storybook):
// - Pure presentational components
// - Simple UI components with minimal logic
//
// When adding features:
// - New hooks with business logic → add unit tests
// - New API integrations → add service tests
// - Complex components → add Storybook stories + optional unit tests
// =============================================================================

// Cleanup after each test case (e.g., clearing jsdom)
afterEach(() => {
  cleanup();
});
