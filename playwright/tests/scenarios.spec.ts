import { test, expect } from '@playwright/test';

// Replication factor: replicate the canonical base scenario set REPS times to
// create enough independent, parallelizable work to measure. Total tests = 33 * REPS
// (A7 + B3 + C5 + D4 + E2 + F3 + G6 + H2). Playwright has a single, always-realistic
// interaction model. The Biloba suite runs this same set twice — through `b`
// (biloba-fast) and `b.Realistic()` (biloba-realistic) — plus a fast-only CSS-hook
// variant of Bucket F.
const REPS = parseInt(process.env.REPS ?? '8', 10);

for (let r = 1; r <= REPS; r++) {
  test.describe(`rep-${r}`, () => {
    // ---- Page A — dom.html (read-only DOM) ---------------------------------
    test.describe('Page A — dom.html', () => {
      test.beforeEach(async ({ page }) => {
        await page.goto('/dom.html');
        // Readiness anchor: wait for #heading before doing anything else.
        await expect(page.locator('#heading')).toBeAttached();
      });

      test('A1 — navigation & title', async ({ page }) => {
        await expect(page).toHaveTitle('DOM Fixture');
        await expect(page.locator('#heading')).toHaveText('Widgets');
      });

      test('A2 — count', async ({ page }) => {
        await expect(page.locator('.item')).toHaveCount(4);
      });

      test('A3 — visibility', async ({ page }) => {
        await expect(page.locator('.item:not(.hidden)')).toHaveCount(3);
        await expect(page.locator('.item.hidden')).toBeHidden();
      });

      test('A4 — inner text of all matches (foreach / all)', async ({ page }) => {
        await expect(page.locator('.item')).toHaveText([
          'Alpha',
          'Bravo',
          'Charlie',
          'Delta',
        ]);
      });

      test('A5 — attribute', async ({ page }) => {
        await expect(page.locator('#status')).toHaveAttribute('data-state', 'ready');
      });

      test('A6 — class', async ({ page }) => {
        await expect(page.locator('#status')).toHaveClass(/(^|\s)muted(\s|$)/);
      });

      test('A7 — property', async ({ page }) => {
        await expect(page.locator('#docs-link')).toHaveJSProperty(
          'href',
          'https://example.com/docs',
        );
      });
    });

    // ---- Page B — interactions.html ----------------------------------------
    test.describe('Page B — interactions.html', () => {
      test.beforeEach(async ({ page }) => {
        await page.goto('/interactions.html');
        await expect(page.locator('#heading')).toBeAttached();
      });

      test('B1 — click (counter)', async ({ page }) => {
        await page.locator('#increment').click();
        await page.locator('#increment').click();
        await page.locator('#increment').click();
        await expect(page.locator('#count')).toHaveText('3');
      });

      test('B2 — form fill (value-set semantics)', async ({ page }) => {
        // value-set path: fill/selectOption/check fire input/change, NOT real keys.
        await page.locator('#name').fill('Jane');
        await page.locator('#role').selectOption('editor');
        await page.locator('#subscribe').check();
        await page.locator('#save').click();
        await expect(page.locator('#result')).toHaveText('Jane / editor / subscribed');
      });

      test('B3 — real keystroke typing (search-as-you-type)', async ({ page }) => {
        // REAL keys via pressSequentially so keyup fires (fill would not).
        await page.locator('#search').pressSequentially('ap');
        await expect(page.locator('.fruit:visible')).toHaveCount(2);
        await expect(page.locator('.fruit:visible')).toHaveText(['Apple', 'Apricot']);
      });
    });

    // ---- Page C — network.html ---------------------------------------------
    test.describe('Page C — network.html', () => {
      test.beforeEach(async ({ page }) => {
        await page.goto('/network.html');
        await expect(page.locator('#heading')).toBeAttached();
      });

      test('C1 — observe a real request', async ({ page }) => {
        await page.locator('#load').click();
        await expect(page.locator('.result')).toHaveCount(3);
        await expect(page.locator('.result')).toHaveText(['One', 'Two', 'Three']);
      });

      test('C2 — stub the request (short-circuit)', async ({ page }) => {
        await page.route('**/api/items', async (route) => {
          await route.fulfill({
            contentType: 'application/json',
            body: JSON.stringify({ items: ['Stubbed'] }),
          });
        });
        await page.locator('#load').click();
        await expect(page.locator('.result')).toHaveCount(1);
        await expect(page.locator('.result')).toHaveText(['Stubbed']);
      });

      test('C3 — wait through latency', async ({ page }) => {
        await page.locator('#load-slow').click();
        // Auto-wait through the fixed 300ms latency.
        await expect(page.locator('.result')).toHaveCount(3);
      });

      test('C4 — abort the request', async ({ page }) => {
        await page.route('**/api/items', (route) => route.abort());
        await page.locator('#load').click();
        await expect(page.locator('.result')).toHaveCount(1);
        await expect(page.locator('.result')).toHaveText(['Error']);
      });

      test('C5 — modify the real response', async ({ page }) => {
        // Hit the REAL server, then rewrite the body (a real round-trip, unlike C2).
        await page.route('**/api/items', async (route) => {
          const response = await route.fetch();
          await route.fulfill({ response, body: JSON.stringify({ items: ['Modified'] }) });
        });
        await page.locator('#load').click();
        await expect(page.locator('.result')).toHaveCount(1);
        await expect(page.locator('.result')).toHaveText(['Modified']);
      });
    });

    // ---- Bucket D — scale (speed-at-scale) ---------------------------------
    test.describe('Bucket D — scale', () => {
      test('D1 — large table render', async ({ page }) => {
        await page.goto('/scale.html');
        await expect(page.locator('#heading')).toBeAttached();
        await expect(page.locator('.row')).toHaveCount(1000);
        await expect(page.locator('.row[data-id="500"] .name')).toHaveText('Item 500');
      });

      test('D2 — filter a large list with real keys', async ({ page }) => {
        await page.goto('/scale.html');
        await expect(page.locator('#heading')).toBeAttached();
        // REAL keys via pressSequentially so keyup fires (fill would not).
        await page.locator('#q').pressSequentially('cat-3');
        // visible = not display:none, 200 of the 1000 rows match cat-3.
        await expect(page.locator('.row:visible')).toHaveCount(200);
      });

      test('D3 — gated multi-step wizard', async ({ page }) => {
        await page.goto('/wizard.html');
        await expect(page.locator('#heading')).toBeAttached();
        // value-set semantics via fill; each click auto-waits for the gated button.
        await page.locator('#input1').fill('a');
        await page.locator('#next1').click();
        await page.locator('#input2').fill('b');
        await page.locator('#next2').click();
        await page.locator('#input3').fill('c');
        await page.locator('#next3').click();
        await page.locator('#input4').fill('d');
        await page.locator('#finish4').click();
        await expect(page.locator('#summary')).toHaveText('a-b-c-d');
      });

      test('D4 — staggered async / eventual consistency', async ({ page }) => {
        await page.goto('/async.html');
        await expect(page.locator('#heading')).toBeAttached();
        await page.locator('#start').click();
        // Auto-wait through the staggered appends (one every 40ms).
        await expect(page.locator('.async-item')).toHaveCount(10);
        await expect(page.locator('.async-item').last()).toHaveText('Item 10');
      });
    });

    // ---- Bucket E — realism (always native on Playwright) ------------------
    test.describe('Bucket E — realism', () => {
      test('E1 — occlusion', async ({ page }) => {
        await page.goto('/occlusion.html');
        await expect(page.locator('#heading')).toBeAttached();
        // Actionability auto-waits until the overlay clears (~250ms) → native.
        await page.locator('#occluded-btn').click();
        await expect(page.locator('#occ-count')).toHaveText('1');
      });

      test('E2 — scroll-into-view', async ({ page }) => {
        await page.goto('/scroll.html');
        await expect(page.locator('#heading')).toBeAttached();
        // Auto-scrolls the below-the-fold button into view, then clicks → native.
        await page.locator('#below-btn').click();
        await expect(page.locator('#scroll-result')).toHaveText('clicked');
      });
    });

    // ---- Bucket F — semantic locators --------------------------------------
    test.describe('Bucket F — semantic locators', () => {
      test.beforeEach(async ({ page }) => {
        await page.goto('/locators.html');
        await expect(page.locator('#heading')).toBeAttached();
      });

      test('F1 — role + name', async ({ page }) => {
        await page.getByRole('button', { name: 'Save', exact: true }).click();
        await expect(page.locator('#save-result')).toHaveText('saved');
      });

      test('F2 — visible text', async ({ page }) => {
        await expect(page.getByText('Featured item', { exact: true })).toBeVisible();
      });

      test('F3 — form label', async ({ page }) => {
        await page.getByLabel('Email').fill('x@y.com');
        await expect(page.getByLabel('Email')).toHaveValue('x@y.com');
      });
    });

    // ---- Bucket G — interaction vocabulary ---------------------------------
    test.describe('Bucket G — interaction vocabulary', () => {
      test.beforeEach(async ({ page }) => {
        await page.goto('/vocab.html');
        await expect(page.locator('#heading')).toBeAttached();
      });

      test('G1 — double-click', async ({ page }) => {
        await page.locator('#dbl-btn').dblclick();
        await expect(page.locator('#dbl-result')).toHaveText('double');
      });

      test('G2 — right-click', async ({ page }) => {
        await page.locator('#ctx-btn').click({ button: 'right' });
        await expect(page.locator('#ctx-result')).toHaveText('menu');
      });

      test('G3 — middle-click', async ({ page }) => {
        await page.locator('#aux-btn').click({ button: 'middle' });
        await expect(page.locator('#aux-result')).toHaveText('middle');
      });

      test('G4 — drag-and-drop', async ({ page }) => {
        await page.locator('#drag-src').dragTo(page.locator('#drop-zone'));
        await expect(page.locator('#drop-result')).toHaveText('dropped');
      });

      test('G5 — wheel scroll', async ({ page }) => {
        await page.locator('#scroll-box').hover();
        await page.mouse.wheel(0, 200);
        await expect(page.locator('#wheel-result')).toHaveText('wheeled');
      });

      // Tap needs a touch-enabled context; scope hasTouch to just this test so the
      // rest of the suite keeps Playwright's default (non-touch) context.
      test.describe('touch', () => {
        test.use({ hasTouch: true });
        test('G6 — tap (touch)', async ({ page }) => {
          await page.locator('#tap-btn').tap();
          await expect(page.locator('#tap-result')).toHaveText('tapped');
        });
      });
    });

    // ---- Bucket H — pointer options ----------------------------------------
    test.describe('Bucket H — pointer options', () => {
      test.beforeEach(async ({ page }) => {
        await page.goto('/pointer.html');
        await expect(page.locator('#heading')).toBeAttached();
      });

      test('H1 — click at offset', async ({ page }) => {
        await page.locator('#click-pad').click({ position: { x: 30, y: 40 } });
        await expect(page.locator('#click-pad-result')).toHaveText('ok');
      });

      test('H2 — modifier-click', async ({ page }) => {
        await page.locator('#mod-btn').click({ modifiers: ['Shift'] });
        await expect(page.locator('#mod-result')).toHaveText('shift');
      });
    });
  });
}
