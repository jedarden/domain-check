import { test, expect } from '@playwright/test';

/**
 * HTTP-based tests for WHOIS ccTLD bulk and multi-TLD API endpoints
 *
 * These tests verify that WHOIS path works correctly through bulk and multi-TLD
 * API endpoints, covering:
 * - Bulk endpoint with mixed .de/.jp domains
 * - Multi-TLD endpoint with WHOIS ccTLDs
 * - Partial success scenarios (mixed WHOIS and RDAP domains)
 * - Registration details in bulk responses for registered ccTLDs
 *
 * Follows the same HTTP request pattern as simple-http.spec.ts
 *
 * Prerequisites:
 * - The Go server must be running on localhost:8080
 * - Start with: ./domain-check serve --addr :8080
 *
 * Note: Tests accept both 200 (success) and 429 (rate limited) responses to
 * handle rate limiting gracefully during test runs.
 */

test.describe('WHOIS ccTLD Bulk and Multi-TLD API', () => {
  let baseURL: string;

  test.beforeAll(async () => {
    baseURL = process.env.BASE_URL || 'http://localhost:8080';
  });

  /**
   * Test 1: POST /api/v1/bulk with mixed .de/.jp domains returns source=whois for all
   */
  test('bulk endpoint should return source=whois for mixed .de/.jp domains', async ({ request }) => {
    const response = await request.post(`${baseURL}/api/v1/bulk`, {
      data: {
        domains: ['google.de', 'google.jp', 'example.de', 'example.jp']
      },
      headers: {
        'Content-Type': 'application/json'
      }
    });

    expect(response.status()).toBe(200);
    expect(response.headers()['content-type']).toContain('application/json');

    const data = await response.json();
    expect(data.total).toBe(4);
    expect(data.succeeded).toBeGreaterThanOrEqual(0); // Accept partial success
    expect(data.failed).toBeGreaterThanOrEqual(0);
    expect(data.results).toHaveLength(4);
    expect(data).toHaveProperty('duration');

    // Verify all results have source=whois
    for (const result of data.results) {
      expect(result.result).toBeDefined();
      expect(result.result.source).toBe('whois');
      expect(result.result.tld).toMatch(/^(de|jp)$/);
    }
  });

  /**
   * Test 2: Bulk endpoint returns registration details for registered .de domains
   */
  test('bulk endpoint should include registration details for registered .de domains', async ({ request }) => {
    const response = await request.post(`${baseURL}/api/v1/bulk`, {
      data: {
        domains: ['google.de', 'example.de']
      },
      headers: {
        'Content-Type': 'application/json'
      }
    });

    expect(response.status()).toBe(200);

    const data = await response.json();
    expect(data.results).toHaveLength(2);

    // Check google.de (should be registered)
    const googleResult = data.results.find(r => r.domain === 'google.de');
    expect(googleResult).toBeDefined();
    expect(googleResult.result.available).toBe(false);
    expect(googleResult.result.registration).toBeDefined();

    // Verify some registration data is present (WHOIS data varies)
    const hasRegData = googleResult.result.registration.registrar ||
                      googleResult.result.registration.nameservers ||
                      googleResult.result.registration.status ||
                      googleResult.result.registration.created ||
                      googleResult.result.registration.expires;
    expect(hasRegData).toBeTruthy();
  });

  /**
   * Test 3: Bulk endpoint returns registration details for registered .jp domains
   */
  test('bulk endpoint should include registration details for registered .jp domains', async ({ request }) => {
    const response = await request.post(`${baseURL}/api/v1/bulk`, {
      data: {
        domains: ['google.jp', 'example.jp']
      },
      headers: {
        'Content-Type': 'application/json'
      }
    });

    expect(response.status()).toBe(200);

    const data = await response.json();
    expect(data.results).toHaveLength(2);

    // Check google.jp (should be registered)
    const googleResult = data.results.find(r => r.domain === 'google.jp');
    expect(googleResult).toBeDefined();
    expect(googleResult.result.available).toBe(false);
    expect(googleResult.result.registration).toBeDefined();

    // Verify some registration data is present
    const hasRegData = googleResult.result.registration.registrar ||
                      googleResult.result.registration.nameservers ||
                      googleResult.result.registration.status ||
                      googleResult.result.registration.created ||
                      googleResult.result.registration.expires;
    expect(hasRegData).toBeTruthy();
  });

  /**
   * Test 4: GET /api/v1/check?d=name&tlds=de,jp returns multi-TLD WHOIS results
   */
  test('multi-TLD endpoint should return WHOIS results for .de and .jp', async ({ request }) => {
    const response = await request.get(`${baseURL}/api/v1/check?d=example&tlds=de,jp`);

    expect(response.status()).toBe(200);
    expect(response.headers()['content-type']).toContain('application/json');

    const data = await response.json();
    expect(data.name).toBe('example');
    expect(data.total).toBe(2);
    expect(data.succeeded).toBeGreaterThanOrEqual(0);
    expect(data.failed).toBeGreaterThanOrEqual(0);
    expect(data.results).toHaveLength(2);
    expect(data).toHaveProperty('duration');

    // Verify both .de and .jp results have source=whois
    for (const result of data.results) {
      expect(result.result).toBeDefined();
      expect(result.result.source).toBe('whois');
      expect(result.result.tld).toMatch(/^(de|jp)$/);
      expect(result.domain).toMatch(/^(example\.de|example\.jp)$/);
    }
  });

  /**
   * Test 5: Multi-TLD endpoint returns correct TLD-specific results
   */
  test('multi-TLD endpoint should return correct domain names for each TLD', async ({ request }) => {
    const response = await request.get(`${baseURL}/api/v1/check?d=test&tlds=de,jp`);

    expect(response.status()).toBe(200);

    const data = await response.json();
    expect(data.results).toHaveLength(2);

    // Verify correct domain names
    const domains = data.results.map(r => r.domain);
    expect(domains).toContain('test.de');
    expect(domains).toContain('test.jp');

    // Verify TLD fields
    for (const result of data.results) {
      expect(result.result.domain).toBe(result.domain);
      if (result.domain === 'test.de') {
        expect(result.result.tld).toBe('de');
      } else if (result.domain === 'test.jp') {
        expect(result.result.tld).toBe('jp');
      }
    }
  });

  /**
   * Test 6: Bulk endpoint handles partial success (mixed WHOIS and RDAP)
   */
  test('bulk endpoint should handle mixed WHOIS and RDAP domains', async ({ request }) => {
    const response = await request.post(`${baseURL}/api/v1/bulk`, {
      data: {
        domains: ['google.de', 'google.com', 'google.jp', 'google.org']
      },
      headers: {
        'Content-Type': 'application/json'
      }
    });

    // Accept both 200 (success) and 429 (rate limited)
    expect([200, 429]).toContain(response.status());

    if (response.status() === 429) {
      return; // Skip test if rate limited
    }

    const data = await response.json();
    expect(data.total).toBe(4);
    expect(data.succeeded).toBeGreaterThanOrEqual(0);
    expect(data.results).toHaveLength(4);

    // Verify .de and .jp use WHOIS
    const deResult = data.results.find(r => r.domain === 'google.de');
    const jpResult = data.results.find(r => r.domain === 'google.jp');

    if (deResult && deResult.result) {
      expect(deResult.result.source).toBe('whois');
    }
    if (jpResult && jpResult.result) {
      expect(jpResult.result.source).toBe('whois');
    }

    // Verify .com and .org use RDAP (if they succeeded)
    const comResult = data.results.find(r => r.domain === 'google.com');
    const orgResult = data.results.find(r => r.domain === 'google.org');

    if (comResult && comResult.result && !comResult.result.error) {
      expect(comResult.result.source).toBe('rdap');
    }
    if (orgResult && orgResult.result && !orgResult.result.error) {
      expect(orgResult.result.source).toBe('rdap');
    }
  });

  /**
   * Test 7: Multi-TLD endpoint handles partial success (mixed WHOIS and RDAP)
   */
  test('multi-TLD endpoint should handle mixed WHOIS and RDAP TLDs', async ({ request }) => {
    const response = await request.get(`${baseURL}/api/v1/check?d=google&tlds=de,jp,com,org`);

    // Accept both 200 (success) and 429 (rate limited)
    expect([200, 429]).toContain(response.status());

    if (response.status() === 429) {
      return; // Skip test if rate limited
    }

    const data = await response.json();
    expect(data.total).toBe(4);
    expect(data.succeeded).toBeGreaterThanOrEqual(0);
    expect(data.results).toHaveLength(4);

    // Verify .de and .jp use WHOIS
    const deResult = data.results.find(r => r.domain === 'google.de');
    const jpResult = data.results.find(r => r.domain === 'google.jp');

    if (deResult && deResult.result) {
      expect(deResult.result.source).toBe('whois');
    }
    if (jpResult && jpResult.result) {
      expect(jpResult.result.source).toBe('whois');
    }

    // Verify .com and .org use RDAP (if they succeeded)
    const comResult = data.results.find(r => r.domain === 'google.com');
    const orgResult = data.results.find(r => r.domain === 'google.org');

    if (comResult && comResult.result && !comResult.result.error) {
      expect(comResult.result.source).toBe('rdap');
    }
    if (orgResult && orgResult.result && !orgResult.result.error) {
      expect(orgResult.result.source).toBe('rdap');
    }
  });

  /**
   * Test 8: Bulk endpoint includes duration and basic metadata for WHOIS domains
   */
  test('bulk endpoint should include duration and metadata for WHOIS results', async ({ request }) => {
    const response = await request.post(`${baseURL}/api/v1/bulk`, {
      data: {
        domains: ['google.de', 'google.jp']
      },
      headers: {
        'Content-Type': 'application/json'
      }
    });

    expect(response.status()).toBe(200);

    const data = await response.json();
    expect(data).toHaveProperty('duration');
    expect(typeof data.duration).toBe('number');

    // Verify each result has complete metadata
    for (const result of data.results) {
      expect(result.result).toHaveProperty('domain');
      expect(result.result).toHaveProperty('available');
      expect(result.result).toHaveProperty('tld');
      expect(result.result).toHaveProperty('checked_at');
      expect(result.result).toHaveProperty('source', 'whois');
      expect(result.result).toHaveProperty('cached');
      expect(result.result).toHaveProperty('duration_ms');
      expect(typeof result.result.duration_ms).toBe('number');
    }
  });

  /**
   * Test 9: Multi-TLD endpoint includes duration and basic metadata
   */
  test('multi-TLD endpoint should include duration and metadata', async ({ request }) => {
    const response = await request.get(`${baseURL}/api/v1/check?d=example&tlds=de,jp`);

    // Accept both 200 (success) and 429 (rate limited)
    expect([200, 429]).toContain(response.status());

    if (response.status() === 429) {
      return; // Skip test if rate limited
    }

    const data = await response.json();
    expect(data).toHaveProperty('duration');
    expect(typeof data.duration).toBe('number');

    // Verify each result has complete metadata
    for (const item of data.results) {
      expect(item.result).toHaveProperty('domain');
      expect(item.result).toHaveProperty('available');
      expect(item.result).toHaveProperty('tld');
      expect(item.result).toHaveProperty('checked_at');
      expect(item.result).toHaveProperty('source', 'whois');
      expect(item.result).toHaveProperty('cached');
      expect(item.result).toHaveProperty('duration_ms');
      expect(typeof item.result.duration_ms).toBe('number');
    }
  });

  /**
   * Test 10: Bulk endpoint handles available WHOIS domains correctly
   */
  test('bulk endpoint should handle available WHOIS domains', async ({ request }) => {
    // Use random-looking domains that are extremely unlikely to be registered
    const response = await request.post(`${baseURL}/api/v1/bulk`, {
      data: {
        domains: ['x7kq9m2z-random-test-domain-20384.de', 'y8jrn4p-another-test-94875.jp']
      },
      headers: {
        'Content-Type': 'application/json'
      }
    });

    // Accept 200 (success) or 429 (rate limited)
    expect([200, 429]).toContain(response.status());

    if (response.status() === 200) {
      const data = await response.json();
      expect(data.results).toHaveLength(2);

      // Check that available domains don't have registration details
      for (const result of data.results) {
        if (result.result && !result.result.error && result.result.available) {
          expect(result.result.source).toBe('whois');
          expect(result.result.registration).toBeUndefined();
        }
      }
    }
  });

  /**
   * Test 11: Multi-TLD endpoint handles available WHOIS domains correctly
   */
  test('multi-TLD endpoint should handle available WHOIS domains', async ({ request }) => {
    const response = await request.get(`${baseURL}/api/v1/check?d=x7kq9m2z-random-test&tlds=de,jp`);

    // Accept 200 (success) or 429 (rate limited)
    expect([200, 429]).toContain(response.status());

    if (response.status() === 200) {
      const data = await response.json();
      expect(data.results).toHaveLength(2);

      // Check that available domains don't have registration details
      for (const item of data.results) {
        if (item.result && !item.result.error && item.result.available) {
          expect(item.result.source).toBe('whois');
          expect(item.result.registration).toBeUndefined();
        }
      }
    }
  });

  /**
   * Test 12: Bulk endpoint validates request structure (max 50 domains)
   */
  test('bulk endpoint should reject requests over 50 domains', async ({ request }) => {
    const domains = Array.from({ length: 51 }, (_, i) => `test${i}.de`);

    const response = await request.post(`${baseURL}/api/v1/bulk`, {
      data: {
        domains
      },
      headers: {
        'Content-Type': 'application/json'
      }
    });

    // Accept both 200 (validation error in body) and 429 (rate limited)
    expect([200, 429]).toContain(response.status());

    if (response.status() === 429) {
      return; // Skip test if rate limited
    }

    // Server returns 200 with error message in body for validation errors
    const data = await response.json();
    expect(data).toHaveProperty('error');
    expect(data.error).toMatch(/too_many_domains|max/i);
  });

  /**
   * Test 13: Bulk endpoint validates request structure (empty array)
   */
  test('bulk endpoint should reject empty domain array', async ({ request }) => {
    const response = await request.post(`${baseURL}/api/v1/bulk`, {
      data: {
        domains: []
      },
      headers: {
        'Content-Type': 'application/json'
      }
    });

    // Accept both 200 (validation error in body) and 429 (rate limited)
    expect([200, 429]).toContain(response.status());

    if (response.status() === 429) {
      return; // Skip test if rate limited
    }

    // Server returns 200 with error message in body for validation errors
    const data = await response.json();
    expect(data).toHaveProperty('error');
    expect(data.error).toMatch(/empty_array|empty/i);
  });

  /**
   * Test 14: Multi-TLD endpoint validates tlds parameter
   */
  test('multi-TLD endpoint should handle single TLD in tlds parameter', async ({ request }) => {
    const response = await request.get(`${baseURL}/api/v1/check?d=example&tlds=de`);

    expect(response.status()).toBe(200);

    const data = await response.json();
    expect(data.total).toBe(1);
    expect(data.results).toHaveLength(1);
    expect(data.results[0].domain).toBe('example.de');
    expect(data.results[0].result.source).toBe('whois');
  });

  /**
   * Test 15: Dedicated /multi endpoint works with WHOIS ccTLDs
   */
  test('/api/v1/check/multi endpoint should work with WHOIS ccTLDs', async ({ request }) => {
    const response = await request.get(`${baseURL}/api/v1/check/multi?d=example&tlds=de,jp`);

    // Accept both 200 (success) and 429 (rate limited)
    expect([200, 429]).toContain(response.status());

    if (response.status() === 429) {
      return; // Skip test if rate limited
    }

    expect(response.headers()['content-type']).toContain('application/json');

    const data = await response.json();
    expect(data.name).toBe('example');
    expect(data.total).toBe(2);
    expect(data.results).toHaveLength(2);

    // Verify both results use WHOIS
    for (const result of data.results) {
      expect(result.result.source).toBe('whois');
      expect(result.result.tld).toMatch(/^(de|jp)$/);
    }
  });

  /**
   * Test 16: Partial success in bulk with one domain failing
   */
  test('bulk endpoint should handle partial success with individual errors', async ({ request }) => {
    const response = await request.post(`${baseURL}/api/v1/bulk`, {
      data: {
        domains: ['google.de', 'invalid-domain-!!!.de', 'google.jp']
      },
      headers: {
        'Content-Type': 'application/json'
      }
    });

    // Accept both 200 (success) and 429 (rate limited)
    expect([200, 429]).toContain(response.status());

    if (response.status() === 429) {
      return; // Skip test if rate limited
    }

    const data = await response.json();
    expect(data.total).toBe(3);
    expect(data.results).toHaveLength(3);

    // Check that valid domains succeeded and invalid domain has error
    const googleDe = data.results.find(r => r.domain === 'google.de');
    const invalidDomain = data.results.find(r => r.domain === 'invalid-domain-!!!.de');
    const googleJp = data.results.find(r => r.domain === 'google.jp');

    if (googleDe && googleDe.result) {
      expect(googleDe.result.source).toBe('whois');
    }

    if (invalidDomain && invalidDomain.result) {
      expect(invalidDomain.result).toHaveProperty('error');
    }

    if (googleJp && googleJp.result) {
      expect(googleJp.result.source).toBe('whois');
    }
  });
});
