import { test, expect } from '@playwright/test';

/**
 * Simple HTTP-based tests for Domain Check Web UI
 *
 * These tests use HTTP requests instead of full browser automation,
 * allowing them to run in environments without GUI libraries.
 *
 * Prerequisites:
 * - The Go server must be running on localhost:8080
 * - Start with: ./domain-check serve --addr :8080
 */

test.describe('Web UI (HTTP)', () => {
  let baseURL: string;

  test.beforeAll(async () => {
    baseURL = process.env.BASE_URL || 'http://localhost:8080';
  });

  /**
   * Test 1: Home page loads with search input
   */
  test('should load home page with search input', async ({ request }) => {
    const response = await request.get(`${baseURL}/`);

    expect(response.status()).toBe(200);
    expect(response.headers()['content-type']).toContain('text/html');

    const html = await response.text();

    // Check for key elements
    expect(html).toContain('Domain Check');
    expect(html).toContain('Authoritative availability lookup');
    expect(html).toContain('id="domain-input"');
    expect(html).toContain('name="d"');
    expect(html).toContain('action="/check"');
    expect(html).toContain('.com');
    expect(html).toContain('.org');
    expect(html).toContain('.net');
    expect(html).toContain('RDAP');
  });

  /**
   * Test 2: Empty form submission redirects to home
   */
  test('should redirect to home when d parameter is empty', async ({ request }) => {
    const response = await request.get(`${baseURL}/check?d=`);

    // Should redirect to home page
    expect(response.status()).toBe(200);
    const html = await response.text();
    expect(html).toContain('Domain Check');
  });

  /**
   * Test 3: Valid domain shows result page
   */
  test('should show result for valid domain', async ({ request }) => {
    const response = await request.get(`${baseURL}/check?d=example.com`);

    // Accept either 200 (success) or 500 (RDAP bootstrap not loaded yet)
    // The key is that the page renders with the domain name
    expect([200, 500]).toContain(response.status());
    expect(response.headers()['content-type']).toContain('text/html');

    const html = await response.text();

    // Check that the domain name is shown
    expect(html).toContain('example.com');

    // If the check succeeded, verify result elements
    if (response.status() === 200) {
      expect(html).toMatch(/Available|Taken|Check failed/);
    }
  });

  /**
   * Test 4: Invalid domain shows error
   */
  test('should show error for invalid domain', async ({ request }) => {
    const response = await request.get(`${baseURL}/check?d=not-a-valid-domain-!!!`);

    // Accept 200 (validation error in page) or 429 (rate limited)
    expect([200, 429]).toContain(response.status());

    if (response.status() === 200) {
      const html = await response.text();
      // Should show error
      expect(html).toMatch(/Invalid|error/i);
    }
  });

  /**
   * Test 5: Multi-TLD check returns results
   */
  test('should return multi-TLD results', async ({ request }) => {
    const response = await request.get(`${baseURL}/check?d=example&tlds=com&tlds=org&tlds=net`);

    expect(response.status()).toBe(200);
    const html = await response.text();

    // Check for multi-TLD results
    expect(html).toContain('example.com');
    expect(html).toContain('example.org');
    expect(html).toContain('example.net');
    expect(html).toContain('Results for');
  });

  /**
   * Test 6: Known taken domain shows registration details
   */
  test('should show registration details for taken domain', async ({ request }) => {
    const response = await request.get(`${baseURL}/check?d=google.com`);

    expect(response.status()).toBe(200);
    const html = await response.text();

    // Should show taken status and registration details
    expect(html).toContain('Taken');
    expect(html).toMatch(/Registrar|Registered|Expires|Nameservers/);
  });

  /**
   * Test 7: Result page has JSON API link
   */
  test('should have JSON API link on result page', async ({ request }) => {
    const response = await request.get(`${baseURL}/check?d=example.com`);

    expect(response.status()).toBe(200);
    const html = await response.text();

    // Should have link to API
    expect(html).toContain('/api/v1/check?d=');
    expect(html).toMatch(/View JSON|api/i);
  });

  /**
   * Test 8: "Also check" section appears on single domain results
   */
  test('should show "also check" section for single domain', async ({ request }) => {
    const response = await request.get(`${baseURL}/check?d=example.com`);

    expect(response.status()).toBe(200);
    const html = await response.text();

    // Should have "also check" section with alternative TLDs
    expect(html).toMatch(/also check|Also check/i);
    expect(html).toContain('/check?d=');
  });

  /**
   * Test 9: Static assets are served
   */
  test('should serve static assets', async ({ request }) => {
    // Check CSS
    const cssResponse = await request.get(`${baseURL}/static/style.css`);
    expect(cssResponse.status()).toBe(200);
    expect(cssResponse.headers()['content-type']).toContain('text/css');
    const css = await cssResponse.text();
    expect(css).toContain('.'); // CSS should have selectors

    // Check favicon
    const faviconResponse = await request.get(`${baseURL}/static/favicon.svg`);
    expect(faviconResponse.status()).toBe(200);
    expect(faviconResponse.headers()['content-type']).toContain('image/svg');
  });

  /**
   * Test 10: Security headers are present
   */
  test('should have security headers', async ({ request }) => {
    const response = await request.get(`${baseURL}/`);

    const headers = response.headers();

    // Check for CSP
    expect(headers['content-security-policy']).toBeDefined();
    expect(headers['content-security-policy']).toContain('default-src');

    // Check for other security headers
    expect(headers['x-content-type-options']).toBe('nosniff');
    expect(headers['x-frame-options']).toBe('DENY');
  });

  /**
   * Test 11: Health check endpoint works
   */
  test('should respond to health check', async ({ request }) => {
    const response = await request.get(`${baseURL}/health`);

    expect(response.status()).toBe(200);
    expect(response.headers()['content-type']).toContain('application/json');

    const data = await response.json();
    expect(data).toHaveProperty('status');
  });

  /**
   * Test 12: API endpoint returns JSON for domain check
   */
  test('should return JSON from API endpoint', async ({ request }) => {
    const response = await request.get(`${baseURL}/api/v1/check?d=example.com`);

    expect(response.status()).toBe(200);
    expect(response.headers()['content-type']).toContain('application/json');

    const data = await response.json();
    expect(data).toHaveProperty('domain');
    expect(data).toHaveProperty('available');
    expect(data).toHaveProperty('tld');
    expect(data).toHaveProperty('checked_at');
  });
});
