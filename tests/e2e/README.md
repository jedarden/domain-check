# Domain Check E2E Tests

End-to-end browser tests for the Domain Check web interface using [Playwright](https://playwright.dev/).

## Test Coverage

The tests validate the following scenarios:

1. **Home page loads** — Verifies search input, TLD checkboxes, and informational content
2. **Empty form submission** — Ensures validation prevents empty submissions
3. **Valid domain without JS** — Tests server-side rendering and redirect to `/check?d=...`
4. **Valid domain with JS** — Tests progressive enhancement with JavaScript
5. **Available domain** — Verifies green "Available" indicator
6. **Taken domain** — Verifies red "Taken" indicator with registration details
7. **Multi-TLD checkboxes** — Tests checking a name across multiple TLDs
8. **Mobile viewport** — Tests responsive layout at 390×844 (iPhone)
9. **Shareable result URLs** — Verifies reloading shows the same result

Additional tests cover error handling, "also check" section, API links, and responsive behavior.

## Prerequisites

1. **Install Node.js dependencies:**
   ```bash
   npm install
   ```

2. **Build and start the Go server:**
   ```bash
   go build -o domain-check ./cmd/domain-check
   ./domain-check serve --port 8080
   ```

   The server must be running on `http://localhost:8080`.

## Running Tests

### Run all tests (headless):
```bash
npm run test:e2e
```

### Run tests with UI (recommended for development):
```bash
npm run test:e2e:ui
```

### Run tests in headed mode (see browser):
```bash
npm run test:e2e:headed
```

### Debug tests:
```bash
npm run test:e2e:debug
```

### Run specific test file:
```bash
npx playwright test web-ui.spec.ts
```

### Run specific test:
```bash
npx playwright test -g "should load home page"
```

## Test Configuration

The `playwright.config.ts` file configures:

- **Base URL:** `http://localhost:8080`
- **Browsers:** Chromium, Firefox, WebKit, Mobile Chrome, Mobile Safari
- **Retries:** 2 in CI, 0 locally
- **Traces:** On first retry
- **Screenshots:** Only on failure

## Continuous Integration

In CI, the tests should run after building the Go binary. Example:

```yaml
- name: Build Go binary
  run: go build -o domain-check ./cmd/domain-check

- name: Start server in background
  run: ./domain-check serve --port 8080 &

- name: Install Node dependencies
  run: npm install

- name: Run E2E tests
  run: npm run test:e2e
```

## Test Isolation

Each test runs in isolation with a fresh browser context. Tests use `beforeEach` to navigate to the home page before running.

## Writing New Tests

Add new test files to `tests/e2e/` with the `.spec.ts` extension. Example:

```typescript
import { test, expect } from '@playwright/test';

test('my new test', async ({ page }) => {
  await page.goto('/');
  await expect(page.locator('h1')).toContainText('Domain Check');
});
```

## Resources

- [Playwright Documentation](https://playwright.dev/)
- [Playwright Best Practices](https://playwright.dev/docs/best-practices)
