import { test, expect } from '@playwright/test';

test.describe('OmniTun Login Page', () => {
  test('login page renders sign-in heading', async ({ page }) => {
    await page.goto('http://localhost:3000/login');
    await expect(page.locator('text=Sign in to OmniTun')).toBeVisible();
  });

  test('login page renders email and password fields', async ({ page }) => {
    await page.goto('http://localhost:3000/login');
    await expect(page.locator('input[type="email"]')).toBeVisible();
    await expect(page.locator('input[type="password"]')).toBeVisible();
    await expect(page.locator('button[type="submit"]')).toBeVisible();
  });

  test('login form shows validation errors on empty submit', async ({ page }) => {
    await page.goto('http://localhost:3000/login');
    await page.locator('button[type="submit"]').click();
    await expect(page.locator('text=Email is required')).toBeVisible();
    await expect(page.locator('text=Password is required')).toBeVisible();
  });

  test('login form shows invalid email error', async ({ page }) => {
    await page.goto('http://localhost:3000/login');
    await page.locator('input[type="email"]').fill('not-an-email');
    await page.locator('input[type="password"]').fill('Password1');
    await page.locator('button[type="submit"]').click();
    await expect(page.locator('text=Invalid email address')).toBeVisible();
  });

  test('login page has navigable register link', async ({ page }) => {
    await page.goto('http://localhost:3000/login');
    const registerLink = page.locator('a:has-text("Register")');
    await expect(registerLink).toBeVisible();
    await expect(registerLink).toHaveAttribute('href', '/register');
  });

  test('login page renders OAuth provider buttons', async ({ page }) => {
    await page.goto('http://localhost:3000/login');
    await expect(page.locator('text=Continue with GitHub')).toBeVisible();
    await expect(page.locator('text=Continue with Google')).toBeVisible();
  });

  test('login page renders SAML and OIDC provider buttons', async ({ page }) => {
    await page.goto('http://localhost:3000/login');
    await expect(page.locator('text=Continue with SAML SSO')).toBeVisible();
    await expect(page.locator('text=Continue with OIDC')).toBeVisible();
  });
});

test.describe('OmniTun Register Page', () => {
  test('register page renders', async ({ page }) => {
    await page.goto('http://localhost:3000/register');
    await expect(page.locator('text=Create your account')).toBeVisible();
  });

  test('register form renders all fields', async ({ page }) => {
    await page.goto('http://localhost:3000/register');
    await expect(page.locator('input[type="text"]')).toBeVisible();
    await expect(page.locator('input[type="email"]')).toBeVisible();
    const passwordFields = page.locator('input[type="password"]');
    await expect(passwordFields).toHaveCount(2);
    await expect(page.locator('button[type="submit"]')).toBeVisible();
  });

  test('register form shows validation errors on empty submit', async ({ page }) => {
    await page.goto('http://localhost:3000/register');
    await page.locator('button[type="submit"]').click();
    await expect(page.locator('text=Name is required')).toBeVisible();
    await expect(page.locator('text=Email is required')).toBeVisible();
    await expect(page.locator('text=Password is required')).toBeVisible();
  });

  test('register form shows password mismatch error', async ({ page }) => {
    await page.goto('http://localhost:3000/register');
    await page.locator('input[type="text"]').fill('Test User');
    await page.locator('input[type="email"]').fill('test@example.com');
    const passwordFields = page.locator('input[type="password"]');
    await passwordFields.nth(0).fill('StrongPass1!');
    await passwordFields.nth(1).fill('DifferentPass2!');
    await page.locator('button[type="submit"]').click();
    await expect(page.locator('text=Passwords do not match')).toBeVisible();
  });

  test('register page has navigable login link', async ({ page }) => {
    await page.goto('http://localhost:3000/register');
    const loginLink = page.locator('a:has-text("Sign in")');
    await expect(loginLink).toBeVisible();
    await expect(loginLink).toHaveAttribute('href', '/login');
  });
});

test.describe('OmniTun Dashboard', () => {
  test('dashboard redirects to login when unauthenticated', async ({ page }) => {
    await page.goto('http://localhost:3000/');
    await page.waitForURL('**/login**', { timeout: 5000 });
    await expect(page.locator('text=Sign in to OmniTun')).toBeVisible();
  });
});

test.describe('OmniTun Language Toggle', () => {
  test('language toggle switches to Chinese', async ({ page }) => {
    await page.goto('http://localhost:3000/login');
    await page.click('button:has-text("中文")');
    await expect(page.locator('text=登录 OmniTun')).toBeVisible();
  });

  test('language toggle switches back to English', async ({ page }) => {
    await page.goto('http://localhost:3000/login');
    await page.click('button:has-text("中文")');
    await page.waitForSelector('text=登录 OmniTun');
    await page.click('button:has-text("EN")');
    await expect(page.locator('text=Sign in to OmniTun')).toBeVisible();
  });
});

test.describe('OmniTun Authenticated Flows', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('http://localhost:3000/login');
    await page.locator('input[type="email"]').fill('admin@omnitun.io');
    await page.locator('input[type="password"]').fill('Admin123!');
    await page.locator('button[type="submit"]').click();
    await page.waitForURL('**/');
  });

  test('dashboard loads after login', async ({ page }) => {
    await expect(page.locator('text=Dashboard')).toBeVisible();
    await expect(page.locator('text=Active Tunnels')).toBeVisible();
  });

  test('navigate to tunnels page', async ({ page }) => {
    await page.click('a[href="/tunnels"]');
    await expect(page.locator('text=Tunnels')).toBeVisible();
    await expect(page.locator('text=Manage your tunnel configurations')).toBeVisible();
  });

  test('navigate to settings page', async ({ page }) => {
    await page.click('a[href="/settings"]');
    await expect(page.locator('text=Settings')).toBeVisible();
    await expect(page.locator('text=Manage your organization and API keys')).toBeVisible();
  });

  test('navigate to domains page', async ({ page }) => {
    await page.click('a[href="/domains"]');
    await expect(page.locator('text=Custom Domains')).toBeVisible();
  });

  test('navigate to networks page', async ({ page }) => {
    await page.click('a[href="/networks"]');
    await expect(page.locator('text=Mesh Networks')).toBeVisible();
  });

  test('navigate to billing page', async ({ page }) => {
    await page.click('a[href="/billing"]');
    await expect(page.locator('text=Billing & Plans')).toBeVisible();
  });

  test('sign out returns to login page', async ({ page }) => {
    await page.click('button:has-text("Sign out")');
    await expect(page).toHaveURL(/\/login/);
    await expect(page.locator('text=Sign in to OmniTun')).toBeVisible();
  });
});

test.describe('OmniTun Tunnel CRUD', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('http://localhost:3000/login');
    await page.locator('input[type="email"]').fill('admin@omnitun.io');
    await page.locator('input[type="password"]').fill('Admin123!');
    await page.locator('button[type="submit"]').click();
    await page.waitForURL('**/');
  });

  test('create tunnel dialog opens and shows form fields', async ({ page }) => {
    await page.click('a[href="/tunnels"]');
    await page.waitForSelector('text=Tunnels');
    await page.click('button:has-text("New Tunnel")');

    await expect(page.locator('text=Create Tunnel')).toBeVisible();
    await expect(page.locator('text=Configure a new tunnel to expose your local service.')).toBeVisible();
  });

  test('create tunnel dialog can be cancelled', async ({ page }) => {
    await page.click('a[href="/tunnels"]');
    await page.waitForSelector('text=Tunnels');
    await page.click('button:has-text("New Tunnel")');
    await page.waitForSelector('text=Create Tunnel');
    await page.click('button:has-text("Cancel")');
    await expect(page.locator('text=Create Tunnel')).not.toBeVisible();
  });
});

test.describe('OmniTun Settings Page', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('http://localhost:3000/login');
    await page.locator('input[type="email"]').fill('admin@omnitun.io');
    await page.locator('input[type="password"]').fill('Admin123!');
    await page.locator('button[type="submit"]').click();
    await page.waitForURL('**/');
  });

  test('settings tabs are navigable', async ({ page }) => {
    await page.click('a[href="/settings"]');
    await page.waitForSelector('text=Settings');

    await expect(page.locator('text=Organization')).toBeVisible();
    await expect(page.locator('text=API Keys')).toBeVisible();

    await page.click('text=Security');
    await expect(page.locator('text=Two-Factor Authentication')).toBeVisible();

    await page.click('text=API Keys');
    await expect(page.locator('text=Manage keys for API and CLI access')).toBeVisible();
  });

  test('organization tab shows user info and usage', async ({ page }) => {
    await page.click('a[href="/settings"]');
    await page.waitForSelector('text=Settings');

    await expect(page.locator('text=Organization Details')).toBeVisible();
    await expect(page.locator('text=Usage')).toBeVisible();
  });
});

test.describe('OmniTun Forgot Password', () => {
  test('forgot password link navigates to reset page', async ({ page }) => {
    await page.goto('http://localhost:3000/login');
    await page.click('a:has-text("Forgot password?")');
    await expect(page).toHaveURL(/\/forgot-password/);
    await expect(page.locator('text=Reset Password')).toBeVisible();
  });

  test('forgot password page has back to login link', async ({ page }) => {
    await page.goto('http://localhost:3000/forgot-password');
    await expect(page.locator('a:has-text("Back to login")')).toBeVisible();
  });
});

test.describe('OmniTun Responsive Layout', () => {
  test('mobile viewport shows menu toggle button', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto('http://localhost:3000/login');
    await page.locator('input[type="email"]').fill('admin@omnitun.io');
    await page.locator('input[type="password"]').fill('Admin123!');
    await page.locator('button[type="submit"]').click();
    await page.waitForURL('**/');

    await expect(page.locator('svg.lucide-menu')).toBeVisible();
  });
});

test.describe('OmniTun E2E Full User Journey', () => {
  test('register, login, navigate, and logout', async ({ page }) => {
    const uniqueId = Date.now();

    // 1. Register
    await page.goto('http://localhost:3000/register');
    await expect(page.locator('text=Create your account')).toBeVisible();
    await page.locator('input[type="text"]').fill('E2E Test User');
    await page.locator('input[type="email"]').fill(`e2e-${uniqueId}@test.com`);
    const pwFields = page.locator('input[type="password"]');
    await pwFields.nth(0).fill('E2ETest123!');
    await pwFields.nth(1).fill('E2ETest123!');

    // 2. Login
    await page.goto('http://localhost:3000/login');
    await page.locator('input[type="email"]').fill('admin@omnitun.io');
    await page.locator('input[type="password"]').fill('Admin123!');
    await page.locator('button[type="submit"]').click();
    await page.waitForURL('**/');

    // 3. Dashboard
    await expect(page.locator('text=Dashboard')).toBeVisible();

    // 4. Tunnels
    await page.click('a[href="/tunnels"]');
    await expect(page.locator('text=Manage your tunnel configurations')).toBeVisible();

    // 5. Settings
    await page.click('a[href="/settings"]');
    await expect(page.locator('text=Settings')).toBeVisible();

    // 6. Domains
    await page.click('a[href="/domains"]');
    await expect(page.locator('text=Custom Domains')).toBeVisible();

    // 7. Networks
    await page.click('a[href="/networks"]');
    await expect(page.locator('text=Mesh Networks')).toBeVisible();

    // 8. Billing
    await page.click('a[href="/billing"]');
    await expect(page.locator('text=Billing & Plans')).toBeVisible();

    // 9. Logout
    await page.click('button:has-text("Sign out")');
    await expect(page).toHaveURL(/\/login/);
    await expect(page.locator('text=Sign in to OmniTun')).toBeVisible();
  });
});
