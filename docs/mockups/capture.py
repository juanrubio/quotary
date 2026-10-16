#!/usr/bin/env python3
"""Export fictional terminal references with an isolated headless browser.

Run: uv run --with playwright==1.58.0 python docs/mockups/capture.py
Uses installed Chrome; QUOTARY_CHROME can select another Chromium executable.
The HTML also opens directly without Python or Playwright.
"""
import json
import os
from pathlib import Path
from playwright.sync_api import sync_playwright

ROOT = Path(__file__).resolve().parent
SCREENS = [
    ('01-subscription-desk', 'overview', 160, 48, 'dark'),
    ('02-overview', 'overview', 120, 40, 'dark'),
    ('03-medium', 'overview', 100, 32, 'dark'),
    ('04-compact', 'overview', 80, 24, 'dark'),
    ('05-small-terminal', 'overview', 60, 20, 'dark'),
    ('06-attention', 'attention', 120, 40, 'dark'),
    ('07-google-allowances', 'detail', 120, 40, 'dark'),
    ('08-muse-history', 'history', 120, 40, 'dark'),
    ('09-source-library', 'picker', 120, 40, 'dark'),
    ('10-daylight', 'overview', 120, 40, 'light'),
    ('11-monochrome', 'attention', 80, 24, 'mono'),
    ('12-first-launch', 'empty', 100, 32, 'dark'),
    ('13-backend-missing', 'backend', 100, 32, 'dark'),
    ('14-keyboard-help', 'help', 100, 32, 'dark'),
    ('15-more-subscriptions', 'overview', 160, 48, 'dark'),
]


def main():
    output = ROOT / 'screens'
    output.mkdir(exist_ok=True)
    manifest = []
    errors = []
    with sync_playwright() as p:
        browser = p.chromium.launch(
            executable_path=os.environ.get('QUOTARY_CHROME', '/usr/bin/google-chrome'),
            headless=True,
        )
        page = browser.new_page(viewport={'width': 1700, 'height': 1100}, device_scale_factor=1)
        page.on('pageerror', lambda error: errors.append(str(error)))
        for name, scene, cols, rows, theme in SCREENS:
            page_number = 2 if name == '15-more-subscriptions' else 1
            page.goto(f'{ROOT.as_uri()}/index.html?capture=1&scene={scene}&size={cols}x{rows}&theme={theme}&page={page_number}')
            page.wait_for_function('Boolean(window.quotaryReference)')
            page.evaluate('document.fonts.ready')
            terminal = page.locator('#terminal')
            text = terminal.inner_text()
            lines = text.split('\n')
            assert len(lines) == rows, (name, len(lines), rows)
            assert all(len(line) == cols for line in lines), name
            assert 'DEMO' in text, name
            terminal.screenshot(path=str(output / f'{name}.png'))
            (output / f'{name}.txt').write_text(text + '\n')
            manifest.append(dict(file=f'{name}.png', text=f'{name}.txt', scene=scene, cols=cols, rows=rows, theme=theme, page=page_number))
            print(f'Captured {name}: {cols}x{rows} {theme}')
        # Check the interactive reference using actual keyboard events.
        page.goto(f'{ROOT.as_uri()}/index.html?capture=1&size=120x40')
        page.locator('#terminal').focus()
        page.keyboard.press('ArrowRight')
        assert page.evaluate('quotaryReference.state().selected') == 'codex:configured'
        page.keyboard.press('Enter')
        assert page.evaluate('quotaryReference.state().view') == 'detail'
        page.keyboard.press('Tab')
        assert page.evaluate('quotaryReference.state().view') == 'history'
        page.keyboard.press('Escape')
        assert page.evaluate('quotaryReference.state().selected') == 'codex:configured'
        page.keyboard.press('/')
        page.keyboard.type('muse')
        page.keyboard.press('Enter')
        assert page.evaluate('quotaryReference.state().selected') == 'muse:configured'
        assert 'Codex' not in page.locator('#terminal').inner_text()
        page.keyboard.press('Escape')
        page.keyboard.press('p')
        assert page.evaluate('quotaryReference.state().view') == 'picker'
        page.keyboard.press('Space')
        assert len(page.evaluate('quotaryReference.state().tracked')) == 23
        page.keyboard.press('a')
        assert len(page.evaluate('quotaryReference.state().tracked')) == 24
        page.keyboard.press('/')
        page.keyboard.type('gemini')
        assert '2/2' in page.locator('#terminal').inner_text()
        page.keyboard.press('Escape')
        page.keyboard.press('Escape')
        page.keyboard.press('R')
        assert page.evaluate('quotaryReference.state().refreshing')
        page.wait_for_function('!quotaryReference.state().refreshing')
        assert page.evaluate('quotaryReference.state().selected') == 'muse:configured'
        # Review harness scaling at small browser sizes must not produce overflow.
        page.goto(f'{ROOT.as_uri()}/index.html')
        for width in [1600, 1000, 390]:
            page.set_viewport_size({'width': width, 'height': 1000})
            page.wait_for_timeout(80)
            assert page.evaluate('document.documentElement.scrollWidth <= window.innerWidth'), width
        # Every screen/size combination remains a complete cell grid.
        for _, scene, _, _, _ in SCREENS:
            for cols, rows in [(160,48),(120,40),(100,32),(80,24),(60,20)]:
                page.evaluate('([s,w,h])=>{quotaryReference.setScene(s);quotaryReference.setSize(w,h)}', [scene,cols,rows])
                text = page.locator('#terminal').inner_text()
                assert len(text.split('\n')) == rows
                assert all(len(line)==cols for line in text.split('\n'))
        assert not errors, errors
        browser.close()
    (output / 'manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')
    print('Keyboard, filtering, source tracking, refresh selection, responsive sizing, and JavaScript checks passed.')


if __name__ == '__main__':
    main()
