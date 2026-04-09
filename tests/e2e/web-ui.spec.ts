import { test, expect } from '@playwright/test';

/**
 * Domain Check Web UI E2E Tests
 *
 * These tests validate the web interface functionality including:
 * - Home page rendering
 * - Form submission (with and without JavaScript)
 * - Domain availability checking
 * - Multi-TLD support
 * - Mobile responsiveness
 * - Shareable result URLs
 *
 * Prerequisites:
 * - The Go server must be running on localhost:8080
 * - Start with: ./domain-check serve --port 8080
 */

test.describe('Web UI', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to home page before each test
    await page.goto('/');
  });

  /**
   * Test 1: Home page loads with search input
   */
  test('should load home page with search input', async ({ page }) => {
    // Check page title
    await expect(page).toHaveTitle(/Domain Check/);

    // Check for main heading
    const heading = page.locator('header h1');
    await expect(heading).toContainText('Domain Check');

    // Check for tagline
    const tagline = page.locator('header .tagline');
    await expect(tagline).toContainText('Authoritative availability lookup');

    // Check for search form
    const form = page.locator('.search-form');
    await expect(form).toBeVisible();

    // Check for domain input
    const input = page.locator('#domain-input');
    await expect(input).toBeVisible();
    await expect(input).toHaveAttribute('placeholder', 'Enter a domain name');
    await expect(input).toHaveAttribute('required');

    // Check for submit button
    const button = page.locator('.search-form button[type="submit"]');
    await expect(button).toBeVisible();
    await expect(button).toHaveAttribute('aria-label', 'Check domain');

    // Check for TLD checkboxes
    const tldOptions = page.locator('.tld-options');
    await expect(tldOptions).toBeVisible();

    const checkboxes = tldOptions.locator('input[type="checkbox"]');
    await expect(checkboxes).toHaveCount(6);

    // Verify specific TLDs
    const tldLabels = ['.com', '.org', '.net', '.dev', '.io', '.app'];
    for (const tld of tldLabels) {
      await expect(tldOptions.locator('label', { hasText: tld })).toBeVisible();
    }

    // Check for "How it works" section
    const howItWorks = page.locator('.info-section');
    await expect(howItWorks).toBeVisible();
    await expect(howItWorks).toContainText('RDAP');
  });

  /**
   * Test 2: Empty form submission shows error
   */
  test('should show error when submitting empty form', async ({ page }) => {
    // Try to submit empty form
    const button = page.locator('.search-form button[type="submit"]');
    await button.click();

    // Browser's HTML5 validation should prevent submission
    // The input should show validation error
    const input = page.locator('#domain-input');
    await expect(input).toBeFocused();

    // Check that we're still on home page (no redirect)
    await expect(page).toHaveURL('/');
  });

  /**
   * Test 3: Valid domain without JS redirects to /check?d=... and shows result
   */
  test('should redirect to check page and show result for valid domain (without JS)', async ({ page }) => {
    // Disable JavaScript to test server-side rendering
    await page.context().setJavaScriptEnabled(false);

    // Fill in domain and submit
    const input = page.locator('#domain-input');
    await input.fill('example.com');

    const button = page.locator('.search-form button[type="submit"]');
    await button.click();

    // Should be on check page with domain parameter
    await expect(page).toHaveURL(/\/check\?d=example\.com/);

    // Re-enable JS for assertions
    await page.context().setJavaScriptEnabled(true);

    // Check that result section is visible
    const resultSection = page.locator('.result-section');
    await expect(resultSection).toBeVisible();

    // Check that domain name is displayed
    const domainName = page.locator('.domain-name');
    await expect(domainName).toBeVisible();
    await expect(domainName).toContainText('example.com');

    // Check for status indicator (available or taken)
    const status = page.locator('.status');
    await expect(status).toBeVisible();
    await expect(status).toMatch(/Available|Taken/);

    // Check for meta information
    const meta = page.locator('.meta');
    await expect(meta).toBeVisible();
    await expect(meta).toContainText('Checked via');
    await expect(meta).toContainText('ms');
  });

  /**
   * Test 4: Valid domain with JS shows inline result
   */
  test('should show inline result with JavaScript enabled', async ({ page }) => {
    // Fill in domain
    const input = page.locator('#domain-input');
    await input.fill('example.com');

    // Submit form
    const button = page.locator('.search-form button[type="submit"]');
    await button.click();

    // Wait for navigation and result display
    await page.waitForURL(/\/check\?d=/);
    await page.waitForLoadState('networkidle');

    // Check that result section is visible
    const resultSection = page.locator('.result-section');
    await expect(resultSection).toBeVisible();

    // Check that domain name is displayed
    const domainName = page.locator('.domain-name');
    await expect(domainName).toContainText('example.com');

    // Check for status
    const status = page.locator('.status');
    await expect(status).toBeVisible();
  });

  /**
   * Test 5: Available domain shows green indicator
   */
  test('should show green indicator for available domain', async ({ page }) => {
    // Use a domain that's likely to be available
    // (Note: in real tests, you'd want to mock the RDAP response)
    const randomDomain = `test-${Date.now()}-${Math.random().toString(36).substring(7)}.com`;

    await page.goto(`/check?d=${randomDomain}`);

    // Wait for result
    await page.waitForSelector('.result-section, .result-card');

    // Check for available status
    const resultCard = page.locator('.result-card.available, .result-card.taken');
    const status = page.locator('.status');

    // The domain should be marked as available or taken
    await expect(status).toBeVisible();

    // If available, check for green styling
    const isAvailable = await page.locator('.status.available').count() > 0;
    if (isAvailable) {
      const availableStatus = page.locator('.status.available');
      await expect(availableStatus).toContainText('Available');

      // Check that result card has available class
      const availableCard = page.locator('.result-card.available');
      await expect(availableCard).toBeVisible();
    }
  });

  /**
   * Test 6: Taken domain shows red indicator + registration details
   */
  test('should show red indicator and registration details for taken domain', async ({ page }) => {
    // Use a known taken domain
    await page.goto('/check?d=google.com');

    // Wait for result
    await page.waitForSelector('.result-section, .result-card');

    // Check for taken status
    const status = page.locator('.status');
    await expect(status).toBeVisible();
    await expect(status).toContainText('Taken');

    // Check that result card has taken class
    const takenCard = page.locator('.result-card.taken');
    await expect(takenCard).toBeVisible();

    // Check for registration details section
    const regDetails = page.locator('.registration-details');
    await expect(regDetails).toBeVisible();

    // Check for registration fields
    await expect(regDetails).toContainText('Registrar');

    // Check for at least some registration data
    const registrar = regDetails.locator('dd').first();
    await expect(registrar).not.toBeEmpty();
  });

  /**
   * Test 7: Multi-TLD checkboxes return results for multiple TLDs
   */
  test('should return results for multiple TLDs when checkboxes are selected', async ({ page }) => {
    // Check multiple TLD checkboxes
    await page.locator('.tld-options input[value="com"]').check();
    await page.locator('.tld-options input[value="org"]').check();
    await page.locator('.tld-options input[value="net"]').check();

    // Fill in domain name (without TLD)
    const input = page.locator('#domain-input');
    await input.fill('example');

    // Submit form
    const button = page.locator('.search-form button[type="submit"]');
    await button.click();

    // Wait for results
    await page.waitForURL(/\/check/);
    await page.waitForLoadState('networkidle');

    // Check for multi-TLD results heading
    const resultsHeading = page.locator('.result-section h2');
    await expect(resultsHeading).toContainText('Results for');

    // Check for multiple result cards
    const resultCards = page.locator('.multi-tld-results .result-card');
    await expect(resultCards).toHaveCount(3);

    // Check that each card has a domain name
    for (const card of await resultCards.all()) {
      const domainName = card.locator('.domain-name');
      await expect(domainName).toBeVisible();
    }

    // Check that all three domains are represented
    await expect(page.locator('.result-card')).toContainText('example.com');
    await expect(page.locator('.result-card')).toContainText('example.org');
    await expect(page.locator('.result-card')).toContainText('example.net');
  });

  /**
   * Test 8: Mobile viewport (390x844) — layout usable, input full-width
   */
  test('should be usable on mobile viewport', async ({ page }) => {
    // Set mobile viewport (iPhone 12 dimensions)
    await page.setViewportSize({ width: 390, height: 844 });

    // Check that page loads properly
    await expect(page.locator('header h1')).toBeVisible();

    // Check that input is full-width (or close to viewport width)
    const input = page.locator('#domain-input');
    await expect(input).toBeVisible();

    const inputBox = await input.boundingBox();
    expect(inputBox?.width).toBeGreaterThan(300); // Should be wide on mobile

    // Check that TLD checkboxes are still usable
    const tldOptions = page.locator('.tld-options');
    await expect(tldOptions).toBeVisible();

    // Verify checkboxes are clickable (not overlapping)
    const firstCheckbox = page.locator('.tld-options input[type="checkbox"]').first();
    await firstCheckbox.check();
    await expect(firstCheckbox).toBeChecked();

    // Check that submit button is visible and clickable
    const button = page.locator('.search-form button[type="submit"]');
    await expect(button).toBeVisible();

    const buttonBox = await button.boundingBox();
    expect(buttonBox).toBeTruthy();
  });

  /**
   * Test 9: Result page URL is shareable (reload shows same result)
   */
  test('should have shareable result URL', async ({ page }) => {
    // Navigate to a specific domain check
    const testDomain = 'example.com';
    await page.goto(`/check?d=${testDomain}`);

    // Wait for result
    await page.waitForSelector('.result-section');

    // Get the current URL
    const currentUrl = page.url();
    expect(currentUrl).toContain(`d=${testDomain}`);

    // Get the domain name from the result
    const domainName = page.locator('.domain-name');
    const firstResult = await domainName.textContent();

    // Reload the page
    await page.reload();

    // Wait for result again
    await page.waitForSelector('.result-section');

    // Check that the same domain is shown
    const reloadedDomainName = page.locator('.domain-name');
    await expect(reloadedDomainName).toContainText(testDomain);

    const reloadedResult = await reloadedDomainName.textContent();
    expect(reloadedResult).toBe(firstResult);

    // Test sharing by copying URL and opening in new context
    const sharedUrl = page.url();

    const newPage = await page.context().newPage();
    await newPage.goto(sharedUrl);

    // Wait for result on new page
    await newPage.waitForSelector('.result-section');

    // Verify same result is shown
    const newPageDomain = newPage.locator('.domain-name');
    await expect(newPageDomain).toContainText(testDomain);

    await newPage.close();
  });

  /**
   * Additional test: Verify error handling for invalid domains
   */
  test('should show error for invalid domain', async ({ page }) => {
    // Navigate with invalid domain
    await page.goto('/check?d=not-a-valid-domain-!!!');

    // Wait for result
    await page.waitForSelector('.result-section, .error');

    // Check for error message
    const errorSection = page.locator('.result-section.error, .error-message');
    await expect(errorSection).toBeVisible();

    // Should show error about invalid domain
    await expect(page.locator('.error-message')).toContainText(/Invalid|error/i);
  });

  /**
   * Additional test: Verify "Also check" section appears for single domain results
   */
  test('should show "also check" section for single domain results', async ({ page }) => {
    // Check a specific domain
    await page.goto('/check?d=example.com');

    // Wait for result
    await page.waitForSelector('.result-section');

    // Check for "also check" section
    const alsoCheck = page.locator('.also-check');
    await expect(alsoCheck).toBeVisible();

    // Check that it has alternative TLD links
    const altTldLinks = alsoCheck.locator('.alt-tld-list a');
    await expect(altTldLinks).toHaveCountGreaterThan(0);

    // Verify links have correct format
    const firstLink = altTldLinks.first();
    await expect(firstLink).toHaveAttribute('href', /\/check\?d=/);
  });

  /**
   * Additional test: Verify JSON API link is present on results
   */
  test('should show JSON API link on result page', async ({ page }) => {
    // Check a domain
    await page.goto('/check?d=example.com');

    // Wait for result
    await page.waitForSelector('.result-section');

    // Check for API link
    const apiLink = page.locator('.api-link a');
    await expect(apiLink).toBeVisible();
    await expect(apiLink).toHaveText(/View JSON/);
    await expect(apiLink).toHaveAttribute('href', '/api/v1/check?d=example.com');
  });

  /**
   * Additional test: Verify responsive behavior across breakpoints
   */
  test('should be responsive across different screen sizes', async ({ page }) => {
    // Test desktop
    await page.setViewportSize({ width: 1920, height: 1080 });
    await expect(page.locator('header h1')).toBeVisible();
    await expect(page.locator('.search-form')).toBeVisible();

    // Test tablet
    await page.setViewportSize({ width: 768, height: 1024 });
    await expect(page.locator('header h1')).toBeVisible();
    await expect(page.locator('.search-form')).toBeVisible();

    // Test mobile
    await page.setViewportSize({ width: 375, height: 667 });
    await expect(page.locator('header h1')).toBeVisible();
    await expect(page.locator('.search-form')).toBeVisible();
  });
});
