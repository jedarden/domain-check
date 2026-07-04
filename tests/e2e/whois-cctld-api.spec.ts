import { test, expect } from '@playwright/test';

/**
 * HTTP-based smoke tests for WHOIS-sourced ccTLD checks (.de and .jp)
 *
 * These tests verify the /api/v1/check endpoint returns correct WHOIS-sourced
 * results for ccTLDs that don't have RDAP servers and fall back to WHOIS.
 *
 * Follows the same HTTP request pattern as simple-http.spec.ts.
 *
 * Prerequisites:
 * - The Go server must be running on localhost:8080
 * - Start with: ./domain-check serve --addr :8080
 */

test.describe('WHOIS ccTLD API (.de and .jp)', () => {
  let baseURL: string;

  test.beforeAll(async () => {
    baseURL = process.env.BASE_URL || 'http://localhost:8080';
  });

  /**
   * Test 1: .de registered domain returns source=whois
   */
  test('should return source=whois for registered .de domain', async ({ request }) => {
    const response = await request.get(`${baseURL}/api/v1/check?d=google.de`);

    expect(response.status()).toBe(200);
    expect(response.headers()['content-type']).toContain('application/json');

    const data = await response.json();
    expect(data.source).toBe('whois');
    expect(data.domain).toBe('google.de');
    expect(data.tld).toBe('de');
    expect(data.available).toBe(false);
    expect(data).toHaveProperty('checked_at');
  });

  /**
   * Test 2: .de registered domain has registration details
   */
  test('should have registration details for registered .de domain', async ({ request }) => {
    const response = await request.get(`${baseURL}/api/v1/check?d=google.de`);

    expect(response.status()).toBe(200);

    const data = await response.json();
    expect(data.available).toBe(false);
    expect(data.registration).toBeDefined();
    expect(data.registration.registrar).toBeTruthy();
  });

  /**
   * Test 3: .de available domain returns available=true
   */
  test('should return available=true for likely-available .de domain', async ({ request }) => {
    // Use a random-looking domain that's extremely unlikely to be registered
    const response = await request.get(`${baseURL}/api/v1/check?d=x7kq9m2z-random-test-domain-20384.de`);

    // Accept 200 (success) or 429 (rate limited)
    expect([200, 429]).toContain(response.status());

    if (response.status() === 200) {
      const data = await response.json();
      expect(data.source).toBe('whois');
      expect(data.domain).toBe('x7kq9m2z-random-test-domain-20384.de');
      expect(data.tld).toBe('de');
      // If the WHOIS lookup succeeds, it should show as available
      if (!data.error) {
        expect(data.available).toBe(true);
        expect(data.registration).toBeUndefined();
      }
    }
  });

  /**
   * Test 4: .jp registered domain returns source=whois
   */
  test('should return source=whois for registered .jp domain', async ({ request }) => {
    const response = await request.get(`${baseURL}/api/v1/check?d=google.jp`);

    expect(response.status()).toBe(200);
    expect(response.headers()['content-type']).toContain('application/json');

    const data = await response.json();
    expect(data.source).toBe('whois');
    expect(data.domain).toBe('google.jp');
    expect(data.tld).toBe('jp');
    expect(data.available).toBe(false);
    expect(data).toHaveProperty('checked_at');
  });

  /**
   * Test 5: .jp registered domain has registration details
   */
  test('should have registration details for registered .jp domain', async ({ request }) => {
    const response = await request.get(`${baseURL}/api/v1/check?d=google.jp`);

    expect(response.status()).toBe(200);

    const data = await response.json();
    expect(data.available).toBe(false);
    expect(data.registration).toBeDefined();
    expect(data.registration.registrar).toBeTruthy();
  });

  /**
   * Test 6: .jp available domain returns available=true
   */
  test('should return available=true for likely-available .jp domain', async ({ request }) => {
    const response = await request.get(`${baseURL}/api/v1/check?d=x7kq9m2z-random-test-domain-20384.jp`);

    // Accept 200 (success) or 429 (rate limited)
    expect([200, 429]).toContain(response.status());

    if (response.status() === 200) {
      const data = await response.json();
      expect(data.source).toBe('whois');
      expect(data.domain).toBe('x7kq9m2z-random-test-domain-20384.jp');
      expect(data.tld).toBe('jp');
      // If the WHOIS lookup succeeds, it should show as available
      if (!data.error) {
        expect(data.available).toBe(true);
        expect(data.registration).toBeUndefined();
      }
    }
  });

  /**
   * Test 7: .de WHOIS result has correct JSON structure
   */
  test('should have complete JSON structure for .de WHOIS result', async ({ request }) => {
    const response = await request.get(`${baseURL}/api/v1/check?d=google.de`);

    expect(response.status()).toBe(200);

    const data = await response.json();
    // Verify all expected fields are present
    expect(data).toHaveProperty('domain', 'google.de');
    expect(data).toHaveProperty('available');
    expect(data).toHaveProperty('tld', 'de');
    expect(data).toHaveProperty('checked_at');
    expect(data).toHaveProperty('source', 'whois');
    expect(data).toHaveProperty('cached');
    expect(data).toHaveProperty('duration_ms');
  });

  /**
   * Test 8: .jp WHOIS result has correct JSON structure
   */
  test('should have complete JSON structure for .jp WHOIS result', async ({ request }) => {
    const response = await request.get(`${baseURL}/api/v1/check?d=google.jp`);

    expect(response.status()).toBe(200);

    const data = await response.json();
    // Verify all expected fields are present
    expect(data).toHaveProperty('domain', 'google.jp');
    expect(data).toHaveProperty('available');
    expect(data).toHaveProperty('tld', 'jp');
    expect(data).toHaveProperty('checked_at');
    expect(data).toHaveProperty('source', 'whois');
    expect(data).toHaveProperty('cached');
    expect(data).toHaveProperty('duration_ms');
  });
});
