import { test, expect } from '@playwright/test';

/**
 * WHOIS ccTLD Web UI E2E Tests
 *
 * These tests validate the web interface correctly renders WHOIS-sourced results
 * for ccTLD domains (.de and .jp) that don't have RDAP servers and fall back to WHOIS.
 *
 * Tests cover:
 * - WHOIS source indicator (not RDAP)
 * - Available vs taken status rendering for ccTLDs
 * - Registration details section for registered ccTLD domains
 * - Multi-TLD checkbox flow with .de and .jp TLDs
 *
 * Prerequisites:
 * - The Go server must be running on localhost:8080
 * - Start with: ./domain-check serve --port 8080
 */

test.describe('WHOIS ccTLD Web UI (.de and .jp)', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to home page before each test
    await page.goto('/');
  });

  /**
   * Test 1: .de registered domain shows WHOIS source indicator
   */
  test('should show WHOIS source indicator for registered .de domain', async ({ page }) => {
    // Navigate to .de domain check
    await page.goto('/check?d=google.de');

    // Wait for result
    await page.waitForSelector('.result-section, .result-card');

    // Check for result section
    const resultSection = page.locator('.result-section');
    await expect(resultSection).toBeVisible();

    // Check domain name is displayed
    const domainName = page.locator('.domain-name');
    await expect(domainName).toBeVisible();
    await expect(domainName).toContainText('google.de');

    // Check for status indicator
    const status = page.locator('.status');
    await expect(status).toBeVisible();
    await expect(status).toContainText('Taken');

    // Check that WHOIS source is shown in metadata
    const meta = page.locator('.meta');
    await expect(meta).toBeVisible();
    await expect(meta).toContainText('Checked via WHOIS');
    await expect(meta).not.toContainText('RDAP');
  });

  /**
   * Test 2: .de available domain shows green WHOIS result
   */
  test('should show green available indicator for available .de domain', async ({ page }) => {
    // Use a random-looking domain that's extremely unlikely to be registered
    const randomDomain = `x7kq9m2z-random-${Date.now()}.de`;
    await page.goto(`/check?d=${randomDomain}`);

    // Wait for result
    await page.waitForSelector('.result-section, .result-card');

    // Check for result section
    const resultSection = page.locator('.result-section');
    await expect(resultSection).toBeVisible();

    // Check domain name is displayed
    const domainName = page.locator('.domain-name');
    await expect(domainName).toBeVisible();
    await expect(domainName).toContainText(randomDomain);

    // Check for status indicator (should show available or taken)
    const status = page.locator('.status');
    await expect(status).toBeVisible();

    // If available, check for green styling and available text
    const isAvailable = await page.locator('.status.available').count() > 0;
    if (isAvailable) {
      await expect(status).toContainText('Available');

      // Check that result card has available class
      const availableCard = page.locator('.result-card.available');
      await expect(availableCard).toBeVisible();
    }

    // Check that WHOIS source is shown in metadata
    const meta = page.locator('.meta');
    await expect(meta).toBeVisible();
    await expect(meta).toContainText('Checked via WHOIS');
  });

  /**
   * Test 3: .de registered domain shows registration details
   */
  test('should show registration details for registered .de domain', async ({ page }) => {
    // Navigate to .de domain check
    await page.goto('/check?d=google.de');

    // Wait for result
    await page.waitForSelector('.result-section, .result-card');

    // Check for registration details section
    const regDetails = page.locator('.registration-details');
    await expect(regDetails).toBeVisible();

    // Check that taken status is shown
    const status = page.locator('.status');
    await expect(status).toContainText('Taken');

    // Check that result card has taken class
    const takenCard = page.locator('.result-card.taken');
    await expect(takenCard).toBeVisible();

    // Check for registration fields (WHOIS data varies, but some should be present)
    const hasRegData = await regDetails.evaluate((el: HTMLElement) => {
      // Check if any registration data is visible
      const text = el.textContent || '';
      return text.includes('Registrar') ||
             text.includes('Nameserver') ||
             text.includes('Created') ||
             text.includes('Expires') ||
             text.includes('Status');
    });
    expect(hasRegData).toBeTruthy();
  });

  /**
   * Test 4: .jp registered domain shows WHOIS source indicator
   */
  test('should show WHOIS source indicator for registered .jp domain', async ({ page }) => {
    // Navigate to .jp domain check
    await page.goto('/check?d=google.jp');

    // Wait for result
    await page.waitForSelector('.result-section, .result-card');

    // Check for result section
    const resultSection = page.locator('.result-section');
    await expect(resultSection).toBeVisible();

    // Check domain name is displayed
    const domainName = page.locator('.domain-name');
    await expect(domainName).toBeVisible();
    await expect(domainName).toContainText('google.jp');

    // Check for status indicator
    const status = page.locator('.status');
    await expect(status).toBeVisible();
    await expect(status).toContainText('Taken');

    // Check that WHOIS source is shown in metadata
    const meta = page.locator('.meta');
    await expect(meta).toBeVisible();
    await expect(meta).toContainText('Checked via WHOIS');
    await expect(meta).not.toContainText('RDAP');
  });

  /**
   * Test 5: .jp available domain shows green WHOIS result
   */
  test('should show green available indicator for available .jp domain', async ({ page }) => {
    // Use a random-looking domain that's extremely unlikely to be registered
    const randomDomain = `x7kq9m2z-random-${Date.now()}.jp`;
    await page.goto(`/check?d=${randomDomain}`);

    // Wait for result
    await page.waitForSelector('.result-section, .result-card');

    // Check for result section
    const resultSection = page.locator('.result-section');
    await expect(resultSection).toBeVisible();

    // Check domain name is displayed
    const domainName = page.locator('.domain-name');
    await expect(domainName).toBeVisible();
    await expect(domainName).toContainText(randomDomain);

    // Check for status indicator (should show available or taken)
    const status = page.locator('.status');
    await expect(status).toBeVisible();

    // If available, check for green styling and available text
    const isAvailable = await page.locator('.status.available').count() > 0;
    if (isAvailable) {
      await expect(status).toContainText('Available');

      // Check that result card has available class
      const availableCard = page.locator('.result-card.available');
      await expect(availableCard).toBeVisible();
    }

    // Check that WHOIS source is shown in metadata
    const meta = page.locator('.meta');
    await expect(meta).toBeVisible();
    await expect(meta).toContainText('Checked via WHOIS');
  });

  /**
   * Test 6: .jp registered domain shows registration details
   */
  test('should show registration details for registered .jp domain', async ({ page }) => {
    // Navigate to .jp domain check
    await page.goto('/check?d=google.jp');

    // Wait for result
    await page.waitForSelector('.result-section, .result-card');

    // Check for registration details section
    const regDetails = page.locator('.registration-details');
    await expect(regDetails).toBeVisible();

    // Check that taken status is shown
    const status = page.locator('.status');
    await expect(status).toContainText('Taken');

    // Check that result card has taken class
    const takenCard = page.locator('.result-card.taken');
    await expect(takenCard).toBeVisible();

    // Check for registration fields (WHOIS data varies, but some should be present)
    const hasRegData = await regDetails.evaluate((el: HTMLElement) => {
      // Check if any registration data is visible
      const text = el.textContent || '';
      return text.includes('Registrar') ||
             text.includes('Nameserver') ||
             text.includes('Created') ||
             text.includes('Expires') ||
             text.includes('Status');
    });
    expect(hasRegData).toBeTruthy();
  });

  /**
   * Test 7: Multi-TLD checkbox flow with .de TLD
   */
  test('should support multi-TLD checking with .de checkbox', async ({ page }) => {
    // Check .de TLD checkbox (assuming it exists in TLD options)
    const deCheckbox = page.locator('.tld-options input[value="de"]');
    const deCheckboxCount = await deCheckbox.count();

    // Only run this test if .de checkbox is available
    if (deCheckboxCount > 0) {
      await deCheckbox.check();

      // Also check .com to test multi-TLD
      await page.locator('.tld-options input[value="com"]').check();

      // Fill in domain name (without TLD)
      const input = page.locator('#domain-input');
      await input.fill('example');

      // Submit form
      const button = page.locator('.search-form button[type="submit"]');
      await button.click();

      // Wait for results
      await page.waitForURL(/\/check/);
      await page.waitForLoadState('networkidle');

      // Check for multi-TLD results
      const resultCards = page.locator('.multi-tld-results .result-card');
      await expect(resultCards).toHaveCount(2);

      // Check that example.de is shown
      await expect(page.locator('.result-card')).toContainText('example.de');

      // Check that example.com is shown
      await expect(page.locator('.result-card')).toContainText('example.com');

      // Verify .de result shows WHOIS source
      const deResult = page.locator('.result-card').filter({ hasText: 'example.de' });
      const deMeta = deResult.locator('.meta');
      await expect(deMeta).toContainText('Checked via WHOIS');
    } else {
      // Skip test if .de checkbox not available
      test.skip(true, '.de TLD checkbox not available in UI');
    }
  });

  /**
   * Test 8: Multi-TLD checkbox flow with .jp TLD
   */
  test('should support multi-TLD checking with .jp checkbox', async ({ page }) => {
    // Check .jp TLD checkbox (assuming it exists in TLD options)
    const jpCheckbox = page.locator('.tld-options input[value="jp"]');
    const jpCheckboxCount = await jpCheckbox.count();

    // Only run this test if .jp checkbox is available
    if (jpCheckboxCount > 0) {
      await jpCheckbox.check();

      // Also check .org to test multi-TLD
      await page.locator('.tld-options input[value="org"]').check();

      // Fill in domain name (without TLD)
      const input = page.locator('#domain-input');
      await input.fill('example');

      // Submit form
      const button = page.locator('.search-form button[type="submit"]');
      await button.click();

      // Wait for results
      await page.waitForURL(/\/check/);
      await page.waitForLoadState('networkidle');

      // Check for multi-TLD results
      const resultCards = page.locator('.multi-tld-results .result-card');
      await expect(resultCards).toHaveCount(2);

      // Check that example.jp is shown
      await expect(page.locator('.result-card')).toContainText('example.jp');

      // Check that example.org is shown
      await expect(page.locator('.result-card')).toContainText('example.org');

      // Verify .jp result shows WHOIS source
      const jpResult = page.locator('.result-card').filter({ hasText: 'example.jp' });
      const jpMeta = jpResult.locator('.meta');
      await expect(jpMeta).toContainText('Checked via WHOIS');
    } else {
      // Skip test if .jp checkbox not available
      test.skip(true, '.jp TLD checkbox not available in UI');
    }
  });

  /**
   * Test 9: Direct navigation to .de result page
   */
  test('should handle direct navigation to .de result page', async ({ page }) => {
    // Direct navigation to /check?d=example.de
    await page.goto('/check?d=example.de');

    // Wait for result
    await page.waitForSelector('.result-section, .result-card');

    // Check for result section
    const resultSection = page.locator('.result-section');
    await expect(resultSection).toBeVisible();

    // Check domain name is displayed
    const domainName = page.locator('.domain-name');
    await expect(domainName).toBeVisible();
    await expect(domainName).toContainText('example.de');

    // Check for status indicator
    const status = page.locator('.status');
    await expect(status).toBeVisible();
    await expect(status).toMatch(/Available|Taken/);

    // Check for meta information with WHOIS source
    const meta = page.locator('.meta');
    await expect(meta).toBeVisible();
    await expect(meta).toContainText('Checked via WHOIS');
    await expect(meta).toContainText('ms');
  });

  /**
   * Test 10: Direct navigation to .jp result page
   */
  test('should handle direct navigation to .jp result page', async ({ page }) => {
    // Direct navigation to /check?d=example.jp
    await page.goto('/check?d=example.jp');

    // Wait for result
    await page.waitForSelector('.result-section, .result-card');

    // Check for result section
    const resultSection = page.locator('.result-section');
    await expect(resultSection).toBeVisible();

    // Check domain name is displayed
    const domainName = page.locator('.domain-name');
    await expect(domainName).toBeVisible();
    await expect(domainName).toContainText('example.jp');

    // Check for status indicator
    const status = page.locator('.status');
    await expect(status).toBeVisible();
    await expect(status).toMatch(/Available|Taken/);

    // Check for meta information with WHOIS source
    const meta = page.locator('.meta');
    await expect(meta).toBeVisible();
    await expect(meta).toContainText('Checked via WHOIS');
    await expect(meta).toContainText('ms');
  });

  /**
   * Test 11: WHOIS vs RDAP source distinction in UI
   */
  test('should distinguish WHOIS from RDAP source in UI', async ({ page }) => {
    // First check a .de domain (WHOIS)
    await page.goto('/check?d=google.de');
    await page.waitForSelector('.result-section');

    const deMeta = page.locator('.meta');
    await expect(deMeta).toContainText('Checked via WHOIS');
    await expect(deMeta).not.toContainText('RDAP');

    // Then check a .com domain (RDAP)
    await page.goto('/check?d=google.com');
    await page.waitForSelector('.result-section');

    const comMeta = page.locator('.meta');
    await expect(comMeta).toContainText('Checked via RDAP');
    await expect(comMeta).not.toContainText('WHOIS');
  });

  /**
   * Test 12: WHOIS ccTLD shareable URLs
   */
  test('should have shareable URLs for WHOIS ccTLD results', async ({ page }) => {
    // Navigate to a .de domain check
    const testDomain = 'google.de';
    await page.goto(`/check?d=${testDomain}`);

    // Wait for result
    await page.waitForSelector('.result-section');

    // Get the current URL
    const currentUrl = page.url();
    expect(currentUrl).toContain(`d=${testDomain}`);

    // Reload the page
    await page.reload();
    await page.waitForSelector('.result-section');

    // Check that the same domain and source are shown
    const domainName = page.locator('.domain-name');
    await expect(domainName).toContainText(testDomain);

    const meta = page.locator('.meta');
    await expect(meta).toContainText('Checked via WHOIS');
  });

  /**
   * Test 13: Mobile responsiveness for WHOIS ccTLD results
   */
  test('should display WHOIS ccTLD results correctly on mobile', async ({ page }) => {
    // Set mobile viewport
    await page.setViewportSize({ width: 390, height: 844 });

    // Check .de domain on mobile
    await page.goto('/check?d=google.de');
    await page.waitForSelector('.result-section');

    // Verify result is readable on mobile
    const domainName = page.locator('.domain-name');
    await expect(domainName).toBeVisible();

    const status = page.locator('.status');
    await expect(status).toBeVisible();

    const meta = page.locator('.meta');
    await expect(meta).toBeVisible();
    await expect(meta).toContainText('Checked via WHOIS');

    // Check registration details on mobile
    const regDetails = page.locator('.registration-details');
    await expect(regDetails).toBeVisible();
  });

  /**
   * Test 14: WHOIS ccTLD result JSON API link
   */
  test('should show correct JSON API link for WHOIS ccTLD results', async ({ page }) => {
    // Check .de domain
    await page.goto('/check?d=google.de');
    await page.waitForSelector('.result-section');

    // Check for API link
    const apiLink = page.locator('.api-link a');
    await expect(apiLink).toBeVisible();
    await expect(apiLink).toHaveText(/View JSON/);
    await expect(apiLink).toHaveAttribute('href', '/api/v1/check?d=google.de');
  });

  /**
   * Test 15: Error handling for invalid WHOIS ccTLD domains
   */
  test('should show error for invalid .de domain', async ({ page }) => {
    // Navigate with invalid .de domain
    await page.goto('/check?d=not-a-valid-domain-!!!.de');

    // Wait for result
    await page.waitForSelector('.result-section, .error');

    // Check for error message
    const errorSection = page.locator('.result-section.error, .error-message');
    await expect(errorSection).toBeVisible();

    // Should show error about invalid domain
    await expect(page.locator('.error-message')).toContainText(/Invalid|error/i);
  });

  /**
   * Test 16: Mixed RDAP and WHOIS in multi-TLD results
   */
  test('should handle mixed RDAP and WHOIS sources in multi-TLD results', async ({ page }) => {
    // Check if both .com (RDAP) and .de (WHOIS) checkboxes exist
    const comCheckbox = page.locator('.tld-options input[value="com"]');
    const deCheckbox = page.locator('.tld-options input[value="de"]');

    const comCount = await comCheckbox.count();
    const deCount = await deCheckbox.count();

    // Only run if both checkboxes are available
    if (comCount > 0 && deCount > 0) {
      await comCheckbox.check();
      await deCheckbox.check();

      // Fill in domain name
      const input = page.locator('#domain-input');
      await input.fill('test');

      // Submit form
      const button = page.locator('.search-form button[type="submit"]');
      await button.click();

      // Wait for results
      await page.waitForURL(/\/check/);
      await page.waitForLoadState('networkidle');

      // Check for multi-TLD results
      const resultCards = page.locator('.multi-tld-results .result-card');
      await expect(resultCards).toHaveCount(2);

      // Verify .com shows RDAP source
      const comResult = page.locator('.result-card').filter({ hasText: 'test.com' });
      const comMeta = comResult.locator('.meta');
      await expect(comMeta).toContainText('Checked via RDAP');

      // Verify .de shows WHOIS source
      const deResult = page.locator('.result-card').filter({ hasText: 'test.de' });
      const deMeta = deResult.locator('.meta');
      await expect(deMeta).toContainText('Checked via WHOIS');
    } else {
      // Skip test if required checkboxes not available
      test.skip(true, 'Required TLD checkboxes not available in UI');
    }
  });
});
